package timezone

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ParseQueryRange 读查询参数里的时间范围，返回半开区间 [start, end)；没给的一端返回 nil，默认值由调用方定。
//
// 每一端有两种写法，只能选一种：
//   - start_time / end_time：RFC3339 精确时刻。「近 24 小时」这类相对范围用它——
//     按自然日传会把起点那天整天都算进来（2026-10-04 走查：近 24 小时查成了两个自然日）。
//   - start_date / end_date：YYYY-MM-DD，按 timezone 参数（缺省或无效时用服务器时区）解释成当天零点；
//     end_date 含当天，取次日零点（AddDate，夏令时安全）。
//
// 两端都给了时要求 start 早于 end：起止颠倒的查询只会得到空结果，直接报错。
func ParseQueryRange(q url.Values) (start, end *time.Time, err error) {
	userTZ := q.Get("timezone")
	start, err = parseRangeBound(q, "start_time", "start_date", userTZ, false)
	if err != nil {
		return nil, nil, err
	}
	end, err = parseRangeBound(q, "end_time", "end_date", userTZ, true)
	if err != nil {
		return nil, nil, err
	}
	if start != nil && end != nil && !start.Before(*end) {
		return nil, nil, errors.New("invalid time range: start must be earlier than end")
	}
	return start, end, nil
}

func parseRangeBound(q url.Values, timeKey, dateKey, userTZ string, isEnd bool) (*time.Time, error) {
	rawTime := strings.TrimSpace(q.Get(timeKey))
	rawDate := strings.TrimSpace(q.Get(dateKey))
	if rawTime != "" && rawDate != "" {
		return nil, fmt.Errorf("%s and %s cannot both be set", timeKey, dateKey)
	}
	if rawTime != "" {
		t, err := time.Parse(time.RFC3339, rawTime)
		if err != nil {
			return nil, fmt.Errorf("invalid %s format, use RFC3339", timeKey)
		}
		return &t, nil
	}
	if rawDate == "" {
		return nil, nil
	}
	t, err := ParseInUserLocation("2006-01-02", rawDate, userTZ)
	if err != nil {
		return nil, fmt.Errorf("invalid %s format, use YYYY-MM-DD", dateKey)
	}
	if isEnd {
		t = t.AddDate(0, 0, 1)
	}
	return &t, nil
}
