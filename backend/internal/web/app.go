package web

// App 是内嵌前端的应用名，对应构建产物 dist 下的子目录。
// 用户站与管理站是两个独立入口、两份产物，分别由各自监听器托管。
type App string

const (
	AppUser  App = "user"
	AppAdmin App = "admin"
)
