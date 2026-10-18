package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// videoTaskPurgeInterval 过期任务的清理间隔（与调度轨迹同一节奏）。
const videoTaskPurgeInterval = time.Hour

// VideoTaskService 视频任务：调上游适配器提交 / 查询 / 取成片，读写任务表，定期清理过期任务。
// 选号、计费由 handler 编排（与其他网关端点同一套槽位、failover、用量记录）。
type VideoTaskService struct {
	repo      VideoTaskRepository
	gateway   *OpenAIGatewayService
	upstreams map[VideoUpstreamKind]VideoTaskUpstream

	stop chan struct{}
	done chan struct{}
	once sync.Once
}

// NewVideoTaskService 创建服务并启动过期清理协程（Stop 停止）。
func NewVideoTaskService(repo VideoTaskRepository, gateway *OpenAIGatewayService) *VideoTaskService {
	s := &VideoTaskService{
		repo:    repo,
		gateway: gateway,
		upstreams: map[VideoUpstreamKind]VideoTaskUpstream{
			VideoUpstreamGrokOAuth:        grokOAuthVideoUpstream{s: gateway},
			VideoUpstreamXAIRelay:         xaiRelayVideoUpstream{s: gateway},
			VideoUpstreamNewAPIRelay:      newAPIRelayVideoUpstream{s: gateway},
			VideoUpstreamSeedanceOfficial: seedanceVideoUpstream{s: gateway},
			VideoUpstreamKlingOfficial:    klingVideoUpstream{s: gateway},
			VideoUpstreamMiniMaxOfficial:  miniMaxVideoUpstream{s: gateway},
		},
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go s.run()
	return s
}

// Stop 停止清理协程。
func (s *VideoTaskService) Stop() {
	if s == nil {
		return
	}
	s.once.Do(func() { close(s.stop) })
	<-s.done
}

func (s *VideoTaskService) run() {
	defer close(s.done)
	ticker := time.NewTicker(videoTaskPurgeInterval)
	defer ticker.Stop()
	s.purge()
	for {
		select {
		case <-ticker.C:
			s.purge()
		case <-s.stop:
			return
		}
	}
}

func (s *VideoTaskService) purge() {
	if s.repo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := s.repo.DeleteVideoTasksBefore(ctx, time.Now().Add(-VideoTaskRetention)); err != nil {
		slog.Warn("video_task.purge_failed", "error", err)
	}
}

// VideoTaskSubmission 一次成功提交：哪个适配器、上游任务 ID、发给上游的模型名，以及实际提交的请求
// （适配器补齐了默认时长 / 分辨率；任务按它落库，没报实际时长 / 分辨率时按它计费）。
type VideoTaskSubmission struct {
	UpstreamKind   VideoUpstreamKind
	UpstreamTaskID string
	UpstreamModel  string
	Request        VideoTaskRequest
}

// videoTaskRequestPreparer 适配器按上游模型校验并补齐请求（时长 / 分辨率默认值、模型支持的取值）；
// 请求不合这个模型时返回 *VideoTaskClientError（不换号：换哪个号模型都一样）。
type videoTaskRequestPreparer interface {
	PrepareRequest(req VideoTaskRequest, upstreamModel string) (VideoTaskRequest, error)
}

// Submit 在选中的账号上提交 entry 的任务。返回的错误：*UpstreamFailoverError（换号重试）、
// *VideoTaskClientError（直接回客户端）、其余（客户端断开等）。上游错误已记进运维事件。
func (s *VideoTaskService) Submit(ctx context.Context, c *gin.Context, account *Account, entry *ModelCatalogEntry, req VideoTaskRequest) (VideoTaskSubmission, error) {
	kind, ok := VideoTaskUpstreamKindFor(VideoTaskVendorFor(entry), account)
	upstream := s.upstreams[kind]
	if !ok || upstream == nil {
		return VideoTaskSubmission{}, &UpstreamFailoverError{StatusCode: http.StatusBadGateway,
			ResponseBody: []byte(fmt.Sprintf(`{"error":{"message":"account %d cannot serve video generation"}}`, account.ID))}
	}
	upstreamModel := strings.TrimSpace(account.GetMappedModel(req.Model))
	if upstreamModel == "" {
		upstreamModel = req.Model
	}
	if preparer, ok := upstream.(videoTaskRequestPreparer); ok {
		prepared, err := preparer.PrepareRequest(req, upstreamModel)
		if err != nil {
			return VideoTaskSubmission{}, err
		}
		req = prepared
	}
	if err := validateVideoEntryTier(entry, req.Resolution); err != nil {
		return VideoTaskSubmission{}, err
	}
	SetOpsUpstreamModel(c, upstreamModel)
	upstreamStart := time.Now()
	id, err := upstream.Create(ctx, c, account, req, upstreamModel)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		return VideoTaskSubmission{}, s.classifyCreateError(ctx, c, account, upstreamModel, err)
	}
	return VideoTaskSubmission{UpstreamKind: kind, UpstreamTaskID: id, UpstreamModel: upstreamModel, Request: req}, nil
}

// classifyCreateError 创建失败的归类与账号副作用：成品号按 Grok 规则（内容审核拒绝不换号），
// key 按 OpenAI 兼容规则；可换号的记 failover 事件，其余记 http_error 并回给客户端。
func (s *VideoTaskService) classifyCreateError(ctx context.Context, c *gin.Context, account *Account, upstreamModel string, err error) error {
	var clientErr *VideoTaskClientError
	var failoverErr *UpstreamFailoverError
	var transportErr *videoUpstreamTransportError
	var httpErr *videoUpstreamHTTPError
	switch {
	case errors.As(err, &clientErr), errors.As(err, &failoverErr):
		return err
	case errors.As(err, &transportErr):
		return s.gateway.handleOpenAIUpstreamTransportError(ctx, c, account, transportErr.Err, false)
	case errors.As(err, &httpErr):
	default:
		if errors.Is(err, context.Canceled) {
			return err
		}
		message := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, message, "")
		appendOpsUpstreamError(c, s.opsEvent(account, 0, "", "", "request_error", message, ""))
		return &UpstreamFailoverError{StatusCode: http.StatusBadGateway,
			ResponseBody: []byte(fmt.Sprintf(`{"error":{"message":%q}}`, message))}
	}

	detail := ""
	if cfg := s.gateway.cfg; cfg != nil && cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		detail = truncateString(string(httpErr.Body), maxBytes)
	}
	failover := false
	if videoAccountKind(account) == videoAccountKindGrokOAuth {
		s.gateway.handleGrokAccountUpstreamError(ctx, account, httpErr.StatusCode, httpErr.Headers, httpErr.Body)
		if isGrokContentPolicyRejection(httpErr.StatusCode, httpErr.Body) {
			message := grokContentPolicyClientMessage(httpErr.Body)
			setOpsUpstreamError(c, httpErr.StatusCode, message, detail)
			appendOpsUpstreamError(c, s.opsEvent(account, httpErr.StatusCode, httpErr.RequestID, httpErr.URL, "http_error", message, detail))
			return &VideoTaskClientError{StatusCode: http.StatusForbidden, Type: "invalid_request_error", Message: message}
		}
		failover = s.gateway.shouldFailoverGrokUpstreamError(httpErr.StatusCode, httpErr.Body)
	} else {
		failover = s.gateway.shouldFailoverOpenAIUpstreamResponse(account, httpErr.StatusCode, httpErr.Message, httpErr.Body)
		if failover {
			s.gateway.handleOpenAIAccountUpstreamError(ctx, account, httpErr.StatusCode, httpErr.Headers, httpErr.Body, upstreamModel)
		}
	}
	kind := "http_error"
	if failover {
		kind = "failover"
	}
	setOpsUpstreamError(c, httpErr.StatusCode, httpErr.Message, detail)
	appendOpsUpstreamError(c, s.opsEvent(account, httpErr.StatusCode, httpErr.RequestID, httpErr.URL, kind, httpErr.Message, detail))
	if failover {
		return &UpstreamFailoverError{StatusCode: httpErr.StatusCode, ResponseBody: httpErr.Body, ResponseHeaders: httpErr.Headers}
	}
	status := httpErr.StatusCode
	if status < 400 || status >= 500 {
		status = http.StatusBadGateway
	}
	return &VideoTaskClientError{StatusCode: status, Type: grokMediaErrorType(status), Message: httpErr.Message}
}

func (s *VideoTaskService) opsEvent(account *Account, status int, requestID, upstreamURL, kind, message, detail string) OpsUpstreamErrorEvent {
	return OpsUpstreamErrorEvent{
		ProxyID:            opsUpstreamProxyID(account),
		ProxyName:          opsUpstreamProxyName(account),
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: status,
		UpstreamRequestID:  requestID,
		UpstreamURL:        upstreamURL,
		Kind:               kind,
		Message:            message,
		Detail:             detail,
	}
}

// CreateTask 落库新任务。
func (s *VideoTaskService) CreateTask(ctx context.Context, task *VideoTask) error {
	return s.repo.CreateVideoTask(ctx, task)
}

// GetTask 按 (id, api key) 取任务；不存在或不属于这把 key 返回 ErrVideoTaskNotFound。
func (s *VideoTaskService) GetTask(ctx context.Context, id string, apiKeyID int64) (*VideoTask, error) {
	return s.repo.GetVideoTask(ctx, strings.TrimSpace(id), apiKeyID)
}

// LoadAccount 任务的承接账号（不经调度器：查询不选号、不记调度轨迹）。
func (s *VideoTaskService) LoadAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s.gateway == nil || s.gateway.accountRepo == nil {
		return nil, errors.New("account repository is unavailable")
	}
	account, err := s.gateway.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, fmt.Errorf("account %d not found", accountID)
	}
	return account, nil
}

// Poll 向承接账号查询一次上游状态，并进任务并写回任务表（终态不再改写）。上游明确说任务不存在
// （400 / 404 / 410）时记失败；其余查询错误原样返回，任务保持原状态，客户端下次再查。
func (s *VideoTaskService) Poll(ctx context.Context, c *gin.Context, task *VideoTask) (VideoTaskResult, *Account, error) {
	account, err := s.LoadAccount(ctx, task.AccountID)
	if err != nil {
		return VideoTaskResult{}, nil, err
	}
	upstream := s.upstreams[task.UpstreamKind]
	if upstream == nil {
		return VideoTaskResult{}, account, fmt.Errorf("unknown video upstream kind %q", task.UpstreamKind)
	}
	result, err := upstream.Query(ctx, c, account, task.UpstreamTaskID)
	if err != nil {
		var httpErr *videoUpstreamHTTPError
		if !errors.As(err, &httpErr) || !isVideoTaskGoneStatus(httpErr.StatusCode) {
			return VideoTaskResult{}, account, err
		}
		result = VideoTaskResult{State: VideoTaskFailed, ErrorCode: "upstream_task_not_found", ErrorMessage: httpErr.Message}
	}
	if !task.State.IsTerminal() {
		task.ApplyResult(result)
		task.UpdatedAt = time.Now()
		if err := s.repo.UpdateVideoTaskResult(ctx, task); err != nil {
			slog.Warn("video_task.update_failed", "task_id", task.ID, "error", err)
		}
	}
	return result, account, nil
}

func isVideoTaskGoneStatus(status int) bool {
	return status == http.StatusBadRequest || status == http.StatusNotFound || status == http.StatusGone
}

// StreamContent 把成片从上游流式转给客户端（不落盘）。result 必须是成功状态。
func (s *VideoTaskService) StreamContent(ctx context.Context, c *gin.Context, task *VideoTask, account *Account, result VideoTaskResult) error {
	upstream := s.upstreams[task.UpstreamKind]
	if upstream == nil {
		return fmt.Errorf("unknown video upstream kind %q", task.UpstreamKind)
	}
	if result.VideoURL == "" {
		result.VideoURL = task.VideoURL
	}
	resp, err := upstream.OpenContent(ctx, c, account, task.UpstreamTaskID, result)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return writeGrokMediaContentResponse(c, resp)
}

// UpstreamEndpoint 用量记录里的上游端点。
func (s *VideoTaskService) UpstreamEndpoint(kind VideoUpstreamKind) string {
	if upstream := s.upstreams[kind]; upstream != nil {
		return upstream.UpstreamEndpoint()
	}
	return ""
}

// ClaimBilling 首次看到成功时抢占计费，只有一个调用方拿到 true。
func (s *VideoTaskService) ClaimBilling(ctx context.Context, id string) (bool, error) {
	return s.repo.ClaimVideoTaskBilling(ctx, id)
}

// ReleaseBilling 入账失败后释放抢占，下一次查询重试。
func (s *VideoTaskService) ReleaseBilling(ctx context.Context, id string) error {
	return s.repo.ReleaseVideoTaskBilling(ctx, id)
}
