// labctl — 机房教学管理系统命令行客户端（P3-1）
//
// 所有功能等价于 Web 管理台，便于脚本化运维与第三方集成。
// 配置优先级：命令行参数 > 环境变量(LABCTL_SERVER/LABCTL_TOKEN) > ~/.labctl.json
//
// 常用示例：
//   labctl login -u admin -p 'ChangeMe@123'
//   labctl agents
//   labctl cmd snapshot e2e-pc-01 e2e-pc-02
//   labctl cmd shutdown e2e-pc-01 --password 'ChangeMe@123'
//   labctl wol aa:bb:cc:dd:ee:ff
//   labctl collect -t "Python作业" --room 1 --pattern '~/Desktop/*.docx'
//   labctl wall start --room 1
//   labctl audits --limit 10
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// ============ 配置 ============

type Config struct {
	Server string `json:"server"`
	Token  string `json:"token"`
	User   string `json:"user,omitempty"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".labctl.json")
}

func loadConfig() Config {
	cfg := Config{Server: "http://127.0.0.1:8090"}
	if b, err := os.ReadFile(configPath()); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if s := os.Getenv("LABCTL_SERVER"); s != "" {
		cfg.Server = s
	}
	if t := os.Getenv("LABCTL_TOKEN"); t != "" {
		cfg.Token = t
	}
	cfg.Server = strings.TrimRight(cfg.Server, "/")
	return cfg
}

func saveConfig(cfg Config) error {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(configPath(), b, 0600)
}

// ============ HTTP 封装 ============

var httpCli = &http.Client{Timeout: 30 * time.Second}

func apiURL(cfg Config, path string) string {
	if strings.HasPrefix(path, "/") {
		return cfg.Server + "/api/v1" + path
	}
	return cfg.Server + "/api/v1/" + path
}

func doJSON(cfg Config, method, path string, body interface{}, out interface{}) (int, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, apiURL(cfg, path), reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	resp, err := httpCli.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return resp.StatusCode, fmt.Errorf("响应解析失败: %s", strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode >= 400 || env.Code != 0 {
		return resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, env.Msg)
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return resp.StatusCode, fmt.Errorf("data 解析失败: %v", err)
		}
	}
	return resp.StatusCode, nil
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "❌ "+format+"\n", args...)
	os.Exit(1)
}

func printTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, r := range rows {
		fmt.Fprintln(w, strings.Join(r, "\t"))
	}
	w.Flush()
}

func timeShort(t string) string {
	if t == "" {
		return "-"
	}
	t = strings.Replace(t, "T", " ", 1)
	if len(t) >= 19 {
		return t[:19]
	}
	return t
}

func jsonPretty(v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

// ============ 子命令实现 ============

func cmdLogin(cfg Config, args []string) {
	var user, pass string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-u", "--user":
			if i+1 < len(args) {
				user = args[i+1]
				i++
			}
		case "-p", "--password":
			if i+1 < len(args) {
				pass = args[i+1]
				i++
			}
		}
	}
	if user == "" {
		user = os.Getenv("LABCTL_USER")
	}
	if pass == "" {
		pass = os.Getenv("LABCTL_PASSWORD")
	}
	if user == "" || pass == "" {
		fatalf("用法: labctl login -u <用户名> -p <密码>")
	}
	var out struct {
		Token string `json:"token"`
	}
	if _, err := doJSON(cfg, "POST", "/auth/login", map[string]string{
		"username": user, "password": pass,
	}, &out); err != nil {
		fatalf("登录失败: %v", err)
	}
	cfg.Token = out.Token
	cfg.User = user
	if err := saveConfig(cfg); err != nil {
		fatalf("保存配置失败: %v", err)
	}
	fmt.Printf("✅ 登录成功（%s@%s），令牌已保存到 %s\n", user, cfg.Server, configPath())
}

func requireToken(cfg Config) {
	if cfg.Token == "" {
		fatalf("未登录。请先执行: labctl login -u <用户名> -p <密码>")
	}
}

func cmdAgents(cfg Config, args []string) {
	requireToken(cfg)
	var out struct {
		OnlineCount int    `json:"online_count"`
		OnlineIDs   []string `json:"online_ids"`
		Sessions    []struct {
			AgentID   string `json:"agent_id"`
			Hostname  string `json:"hostname"`
			IP        string `json:"ip"`
			MAC       string `json:"mac"`
			Online    bool   `json:"online"`
			SeatID    *uint  `json:"seat_id"`
			LastSeen  string `json:"last_seen"`
		} `json:"sessions"`
	}
	if _, err := doJSON(cfg, "GET", "/agents/online", nil, &out); err != nil {
		fatalf("查询失败: %v", err)
	}
	fmt.Printf("在线 %d 台 / 登记 %d 台\n\n", out.OnlineCount, len(out.Sessions))
	rows := [][]string{}
	for _, s := range out.Sessions {
		online := "离线"
		if s.Online {
			online = "● 在线"
		}
		seat := "-"
		if s.SeatID != nil {
			seat = fmt.Sprintf("%d", *s.SeatID)
		}
		rows = append(rows, []string{s.AgentID, online, seat, s.IP, s.MAC, s.Hostname})
	}
	printTable([]string{"AGENT_ID", "状态", "座位", "IP", "MAC", "主机名"}, rows)
}

func cmdRooms(cfg Config, args []string) {
	requireToken(cfg)
	var rooms []struct {
		ID        uint   `json:"id"`
		Name      string `json:"name"`
		Building  string `json:"building"`
		Floor     string `json:"floor"`
		SeatCount int    `json:"seat_count"`
	}
	if _, err := doJSON(cfg, "GET", "/rooms", nil, &rooms); err != nil {
		fatalf("查询失败: %v", err)
	}
	rows := [][]string{}
	for _, r := range rooms {
		rows = append(rows, []string{fmt.Sprintf("%d", r.ID), r.Name, r.Building, r.Floor, fmt.Sprintf("%d", r.SeatCount)})
	}
	printTable([]string{"ID", "机房", "楼栋", "楼层", "座位数"}, rows)
}

func cmdSeats(cfg Config, args []string) {
	requireToken(cfg)
	roomID := flagValue(args, "--room")
	if roomID == "" && len(args) > 0 {
		roomID = args[0]
	}
	if roomID == "" {
		fatalf("用法: labctl seats --room <机房ID>")
	}
	var seats []map[string]interface{}
	if _, err := doJSON(cfg, "GET", "/rooms/"+roomID+"/seats", nil, &seats); err != nil {
		fatalf("查询失败: %v", err)
	}
	rows := [][]string{}
	for _, s := range seats {
		rows = append(rows, []string{
			fmt.Sprint(s["id"]), fmt.Sprint(s["seat_no"]),
			fmt.Sprint(s["student_name"]), fmt.Sprint(s["device_id"]),
		})
	}
	printTable([]string{"ID", "座位号", "学生", "设备ID"}, rows)
}

func cmdCmd(cfg Config, args []string) {
	requireToken(cfg)
	password := flagValue(args, "--password")
	params := map[string]interface{}{}
	if m := flagValue(args, "--params"); m != "" {
		if err := json.Unmarshal([]byte(m), &params); err != nil {
			fatalf("--params 必须是合法 JSON: %v", err)
		}
	}
	rest := positional(args)
	if len(rest) < 2 {
		fatalf("用法: labctl cmd <action> <agent_id...> [--password <密码>] [--params '<json>']\n" +
			"常用 action: lock unlock shutdown reboot snapshot message session_logoff")
	}
	action := rest[0]
	agentIDs := rest[1:]
	var out struct {
		Notified int      `json:"notified"`
		Total    int      `json:"total"`
		MsgIDs   []string `json:"msg_ids"`
	}
	body := map[string]interface{}{
		"agent_ids": agentIDs,
		"action":    action,
		"params":    params,
	}
	if password != "" {
		body["password"] = password
	}
	if _, err := doJSON(cfg, "POST", "/ctrl/command", body, &out); err != nil {
		fatalf("下发失败: %v", err)
	}
	fmt.Printf("✅ 已下发 %s → %d/%d 台在线\n", action, out.Notified, out.Total)
}

func cmdResults(cfg Config, args []string) {
	requireToken(cfg)
	var out interface{}
	if _, err := doJSON(cfg, "GET", "/ctrl/results", nil, &out); err != nil {
		fatalf("查询失败: %v", err)
	}
	list, _ := out.([]interface{})
	if list == nil {
		// 接口实际返回 {items: [...]} 结构
		if m2, ok := out.(map[string]interface{}); ok {
			list, _ = m2["items"].([]interface{})
		}
	}
	rows := [][]string{}
	for _, item := range list {
		m, _ := item.(map[string]interface{})
		rows = append(rows, []string{
			fmt.Sprint(m["id"]), fmt.Sprint(m["agent_id"]), fmt.Sprint(m["action"]),
			fmt.Sprint(m["status"]), fmt.Sprint(m["detail"]), timeShort(fmt.Sprint(m["created_at"])),
		})
	}
	printTable([]string{"ID", "AGENT", "ACTION", "状态", "详情", "时间"}, rows)
}

func cmdWol(cfg Config, args []string) {
	requireToken(cfg)
	rest := positional(args)
	if len(rest) == 0 {
		fatalf("用法: labctl wol <mac地址或agent_id...>")
	}
	body := map[string]interface{}{}
	if strings.Contains(rest[0], ":") || strings.Contains(rest[0], "-") {
		body["mac"] = rest[0]
	} else {
		body["agent_ids"] = rest
	}
	var out struct {
		Sent   int    `json:"sent"`
		Total  int    `json:"total"`
		Detail string `json:"detail"`
	}
	if _, err := doJSON(cfg, "POST", "/wol", body, &out); err != nil {
		fatalf("WOL 失败: %v", err)
	}
	fmt.Printf("✅ magic packet 已发送 %d/%d（%s）\n", out.Sent, out.Total, out.Detail)
}

func cmdCollect(cfg Config, args []string) {
	requireToken(cfg)
	title := flagValue(args, "-t")
	if title == "" {
		title = flagValue(args, "--title")
	}
	room := flagValue(args, "--room")
	pattern := flagValue(args, "--pattern")
	if title == "" || room == "" || pattern == "" {
		fatalf("用法: labctl collect -t <标题> --room <机房ID> --pattern '<文件模式>'")
	}
	roomID, err := strconv.Atoi(room)
	if err != nil {
		fatalf("--room 必须是数字: %s", room)
	}
	var out struct {
		Notified int `json:"notified"`
		Total    int `json:"total"`
		Task     struct {
			ID uint `json:"id"`
		} `json:"task"`
	}
	if _, err := doJSON(cfg, "POST", "/collect/tasks", map[string]interface{}{
		"title": title, "room_id": roomID, "path_pattern": pattern,
	}, &out); err != nil {
		fatalf("发起失败: %v", err)
	}
	fmt.Printf("✅ 收集任务 #%d 已创建，通知 %d/%d 台\n", out.Task.ID, out.Notified, out.Total)
}

func cmdCollectList(cfg Config, args []string) {
	requireToken(cfg)
	var tasks []struct {
		ID          uint   `json:"id"`
		Title       string `json:"title"`
		PathPattern string `json:"path_pattern"`
		Collected   int    `json:"collected"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
	}
	if _, err := doJSON(cfg, "GET", "/collect/tasks", nil, &tasks); err != nil {
		fatalf("查询失败: %v", err)
	}
	rows := [][]string{}
	for _, t := range tasks {
		rows = append(rows, []string{fmt.Sprintf("%d", t.ID), t.Title, t.PathPattern,
			fmt.Sprintf("%d", t.Collected), t.Status, timeShort(t.CreatedAt)})
	}
	printTable([]string{"ID", "标题", "文件模式", "已收集", "状态", "创建时间"}, rows)
}

func cmdPowerTasks(cfg Config, args []string) {
	requireToken(cfg)
	var tasks []struct {
		ID        uint   `json:"id"`
		Name      string `json:"name"`
		Action    string `json:"action"`
		Status    string `json:"status"`
		ExecuteAt string `json:"execute_at"`
		Detail    string `json:"detail"`
	}
	if _, err := doJSON(cfg, "GET", "/power-tasks", nil, &tasks); err != nil {
		fatalf("查询失败: %v", err)
	}
	rows := [][]string{}
	for _, t := range tasks {
		detail := t.Detail
		if len(detail) > 40 {
			detail = detail[:40] + "…"
		}
		rows = append(rows, []string{fmt.Sprintf("%d", t.ID), t.Name, t.Action, t.Status, timeShort(t.ExecuteAt), detail})
	}
	printTable([]string{"ID", "名称", "动作", "状态", "执行时间", "结果"}, rows)
}

func cmdWall(cfg Config, args []string) {
	requireToken(cfg)
	rest := positional(args)
	op := "start"
	if len(rest) > 0 {
		op = rest[0]
	}
	room := flagValue(args, "--room")
	if room == "" {
		fatalf("用法: labctl wall start|stop --room <机房ID>")
	}
	roomID, err := strconv.Atoi(room)
	if err != nil {
		fatalf("--room 必须是数字: %s", room)
	}
	path := "/wall/start"
	if op == "stop" {
		path = "/wall/stop"
	}
	var out struct {
		Notified int `json:"notified"`
		Total    int `json:"total"`
	}
	if _, err := doJSON(cfg, "POST", path, map[string]interface{}{"room_id": roomID}, &out); err != nil {
		fatalf("操作失败: %v", err)
	}
	fmt.Printf("✅ 监控墙 %s：通知 %d/%d 台\n", op, out.Notified, out.Total)
}

func cmdAudits(cfg Config, args []string) {
	requireToken(cfg)
	limit := flagValue(args, "--limit")
	if limit == "" {
		limit = "20"
	}
	var out struct {
		List []struct {
			ID        uint   `json:"id"`
			Action    string `json:"action"`
			Target    string `json:"target"`
			Detail    string `json:"detail"`
			IP        string `json:"ip"`
			CreatedAt string `json:"created_at"`
		} `json:"list"`
	}
	q := url.Values{"limit": {limit}}
	if _, err := doJSON(cfg, "GET", "/audit/logs?"+q.Encode(), nil, &out); err != nil {
		fatalf("查询失败: %v", err)
	}
	rows := [][]string{}
	for _, a := range out.List {
		detail := a.Detail
		if len(detail) > 50 {
			detail = detail[:50] + "…"
		}
		rows = append(rows, []string{fmt.Sprintf("%d", a.ID), a.Action, a.Target, a.IP, detail, timeShort(a.CreatedAt)})
	}
	printTable([]string{"ID", "ACTION", "TARGET", "IP", "详情", "时间"}, rows)
}

func cmdVersion(cfg Config, args []string) {
	fmt.Println("labctl v1.0 (P3)")
	fmt.Println("服务器:", cfg.Server)
	if cfg.Token != "" {
		fmt.Println("认证: 已登录 (" + cfg.User + ")")
	} else {
		fmt.Println("认证: 未登录")
	}
}

// ============ 参数辅助 ============

func flagValue(args []string, name string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}

func positional(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--") || a == "-t" || a == "-u" || a == "-p" {
			i++ // 跳过 flag 的值
			continue
		}
		out = append(out, a)
	}
	return out
}

// ============ 入口 ============

const usage = `labctl — 机房教学管理系统命令行客户端

用法: labctl <命令> [参数]

认证:
  login     -u <用户名> -p <密码>        登录并缓存令牌(~/.labctl.json)
  version                                版本与登录状态

查询:
  agents                                 Agent 列表与在线状态
  rooms                                  机房列表
  seats     --room <机房ID>              座位列表
  results                                指令 ack 结果
  audits    [--limit N]                  审计日志
  collect-list                           作业收集任务列表
  tasks                                  定时电源任务列表

控制:
  cmd <action> <agent_id...> [--password <密码>] [--params '<json>']
          下发指令(lock/unlock/shutdown/reboot/snapshot/message/session_logoff)
  wol <mac或agent_id...>                 远程开机(magic packet)
  collect -t <标题> --room <ID> --pattern '<文件模式>'   发起作业收集
  wall start|stop --room <机房ID>        监控墙控制

环境变量:
  LABCTL_SERVER   服务器地址(默认 http://127.0.0.1:8090)
  LABCTL_TOKEN    直接提供令牌(跳过 login)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}
	cfg := loadConfig()
	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "login":
		cmdLogin(cfg, args)
	case "agents":
		cmdAgents(cfg, args)
	case "rooms":
		cmdRooms(cfg, args)
	case "seats":
		cmdSeats(cfg, args)
	case "cmd":
		cmdCmd(cfg, args)
	case "results":
		cmdResults(cfg, args)
	case "wol":
		cmdWol(cfg, args)
	case "collect":
		cmdCollect(cfg, args)
	case "collect-list":
		cmdCollectList(cfg, args)
	case "tasks":
		cmdPowerTasks(cfg, args)
	case "wall":
		cmdWall(cfg, args)
	case "audits":
		cmdAudits(cfg, args)
	case "version":
		cmdVersion(cfg, args)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", cmd)
		fmt.Print(usage)
		os.Exit(1)
	}
}
