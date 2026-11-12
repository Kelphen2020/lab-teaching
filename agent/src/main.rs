// 机房教学管理系统 - 学生机跨平台 Agent
// 功能：WebSocket 连接 + 注册 + 心跳 + 控制指令 + 屏幕广播 + 远程桌面
// 远程桌面：scrot 截屏 → JPEG → WS binary frame; xdotool 模拟输入
// 跨平台：Windows / Linux / 国产 OS（麒麟/统信/UOS）+ x86_64/arm64/loongarch

use futures_util::{SinkExt, StreamExt};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::io::Write;
use std::sync::Arc;
use std::time::Duration;
use tokio::process::{Child, Command};
use tokio::sync::{mpsc, Mutex};
use tokio::time::interval;
use tokio_tungstenite::{connect_async, tungstenite::Message};

mod platform;

/// 控制通道报文格式 {ver,type,msg_id,ts,payload}
#[derive(Serialize, Deserialize, Debug, Clone)]
struct Frame {
    ver: u32,
    #[serde(rename = "type")]
    ftype: String,
    msg_id: String,
    ts: i64,
    payload: Value,
}

impl Frame {
    fn new(ftype: &str, msg_id: &str, payload: Value) -> Self {
        Frame {
            ver: 1,
            ftype: ftype.to_string(),
            msg_id: msg_id.to_string(),
            ts: now_ts(),
            payload,
        }
    }
}

fn now_ts() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs() as i64)
        .unwrap_or(0)
}

/// Agent 配置（可由环境变量 / 命令行覆盖）
struct Config {
    server_url: String,
    agent_id: String,
    heartbeat_secs: u64,
}

impl Config {
    fn from_env() -> Self {
        Config {
            server_url: std::env::var("LAB_SERVER")
                .unwrap_or_else(|_| "ws://127.0.0.1:8090/agent/ws".to_string()),
            agent_id: std::env::var("LAB_AGENT_ID")
                .unwrap_or_else(|_| default_agent_id()),
            heartbeat_secs: std::env::var("LAB_HEARTBEAT")
                .ok()
                .and_then(|s| s.parse().ok())
                .unwrap_or(10),
        }
    }
}

/// 默认 agent_id：主机名 + MAC 后缀
fn default_agent_id() -> String {
    let host = hostname::get()
        .map(|h| h.to_string_lossy().to_string())
        .unwrap_or_else(|_| "unknown".to_string());
    format!("AGENT-{}", host.to_uppercase())
}

/// 国产操作系统检测（V1.0-11）
/// 解析 /etc/os-release 识别统信UOS / 麒麟Kylin / 中标麒麟NeoKylin
fn detect_os() -> (String, String) {
    let base = std::env::consts::OS.to_string();
    let arch = std::env::consts::ARCH.to_string();
    if base != "linux" {
        return (base, arch);
    }
    // 读取 os-release 识别国产发行版
    let content = std::fs::read_to_string("/etc/os-release")
        .or_else(|_| std::fs::read_to_string("/usr/lib/os-release"))
        .unwrap_or_default();
    let get = |key: &str| -> String {
        content.lines()
            .find(|l| l.starts_with(&format!("{}=", key)))
            .map(|l| l.splitn(2, '=').nth(1).unwrap_or("").trim_matches('"').to_string())
            .unwrap_or_default()
    };
    let id = get("ID").to_lowercase();
    let pretty = get("PRETTY_NAME");
    let distro = if id.contains("uos") || id == "deepin" {
        "统信UOS".to_string()
    } else if id.contains("kylin") {
        if pretty.contains("中标") { "中标麒麟".to_string() } else { "银河麒麟".to_string() }
    } else if !pretty.is_empty() {
        pretty
    } else {
        get("NAME")
    };
    (format!("linux({})", distro), arch)
}

/// 采集本机信息用于注册
fn collect_sysinfo() -> Value {
    let (os, arch) = detect_os();
    json!({
        "os": os,
        "arch": arch,
        "mac": "AA:BB:CC:00:00:01",
        "version": env!("CARGO_PKG_VERSION"),
        "boot_mode": std::env::var("LAB_BOOT_MODE").unwrap_or_else(|_| "本地".to_string()),
    })
}

/// 屏幕广播播放器状态
struct PlayerState {
    child: Option<Child>,
    mcast_addr: String,
}

impl PlayerState {
    fn new() -> Self {
        PlayerState {
            child: None,
            mcast_addr: String::new(),
        }
    }

    async fn start(&mut self, mcast_addr: &str) -> Result<String, String> {
        self.stop().await;
        let url = format!("udp://{}?fifo_size=1000000&overrun_nonfatal=1", mcast_addr);
        println!("[Agent] 启动屏幕广播接收: {}", url);
        let child = Command::new("ffplay")
            .args([
                "-fs", "-probesize", "32768", "-analyzeduration", "0",
                "-fflags", "nobuffer+discardcorrupt", "-flags", "low_delay",
                "-framedrop", "-infbuf", "-i", &url,
            ])
            .kill_on_drop(true)
            .spawn();
        match child {
            Ok(c) => {
                self.child = Some(c);
                self.mcast_addr = mcast_addr.to_string();
                println!("[Agent] ffplay 已启动");
                Ok("ok".to_string())
            }
            Err(e) => {
                let msg = format!("ffplay 启动失败: {}", e);
                println!("[Agent] {}", msg);
                Err(msg)
            }
        }
    }

    async fn stop(&mut self) {
        if let Some(mut child) = self.child.take() {
            println!("[Agent] 停止屏幕广播接收");
            let _ = child.kill().await;
            self.mcast_addr.clear();
        }
    }
}

// ============ P0-1: 屏幕锁定覆盖层（X11 全屏窗口 + 键盘/鼠标抢占） ============
// 覆盖层线程存活于 Agent 进程内：Agent 退出则锁定自动解除（安全兜底）。
// 看门狗：超过 LAB_LOCK_MAX_MIN（默认 240 分钟）自动解除，防止教师遗忘。
struct LockGuard {
    cancel: Option<std::sync::mpsc::Sender<()>>,
    locked_at: i64,
    message: String,
}

impl LockGuard {
    fn new() -> Self {
        LockGuard { cancel: None, locked_at: 0, message: String::new() }
    }
    fn is_locked(&self) -> bool { self.cancel.is_some() }
    fn start(&mut self, message: String) {
        if self.is_locked() { return; }
        let (tx, rx) = std::sync::mpsc::channel::<()>();
        let msg = message.clone();
        std::thread::spawn(move || platform::lock_overlay_main(rx, msg));
        self.cancel = Some(tx);
        self.locked_at = now_ts();
        self.message = message;
        println!("[Agent] 屏幕锁定已启动: {}", self.message);
    }
    fn stop(&mut self) {
        if let Some(tx) = self.cancel.take() {
            let _ = tx.send(());
            println!("[Agent] 屏幕锁定解除信号已发送");
        }
        self.locked_at = 0;
        self.message.clear();
    }
}

/// 远程桌面状态
struct RemoteDesktop {
    /// 截屏循环的取消信号
    cancel: Option<tokio::sync::oneshot::Sender<()>>,
    session_id: u64,
}

/// 监控墙缩略图采集状态（独立于远程桌面，二者可并行）
struct ThumbCapture {
    cancel: Option<tokio::sync::oneshot::Sender<()>>,
}

/// 考试防切屏守卫
struct ExamGuard {
    /// 违规检测循环的取消信号
    cancel: Option<tokio::sync::oneshot::Sender<()>>,
    exam_id: u64,
    anti_cheat: bool,
    /// 考试开始时记录的活动窗口 ID（允许的窗口）
    allowed_window: String,
}

/// 考试状态持久化路径（平台临时目录下：Linux=/tmp，Windows=%TEMP%）
fn exam_state_file() -> String {
    format!("{}/lab-agent-exam-state.json", platform::tmp_dir())
}

impl ExamGuard {
    fn new() -> Self {
        ExamGuard { cancel: None, exam_id: 0, anti_cheat: false, allowed_window: String::new() }
    }

    fn is_active(&self) -> bool {
        self.cancel.is_some()
    }

    /// 启动考试模式：记录当前活动窗口，启动违规检测循环
    fn start(&mut self, exam_id: u64, anti_cheat: bool, agent_id: String, server_base: String) {
        if exam_id == 0 {
            println!("[Agent] ⚠️ exam_id=0 无效，忽略 exam_start");
            return;
        }
        self.stop_sync();
        self.exam_id = exam_id;
        self.anti_cheat = anti_cheat;

        // 记录当前活动窗口作为允许窗口
        let allowed = platform::get_active_window_sync();
        self.allowed_window = allowed.clone();
        println!("[Agent] 考试模式启动: exam_id={} anti_cheat={} allowed_window={}", exam_id, anti_cheat, allowed);

        // 持久化考试状态（断线恢复用）
        save_exam_state(exam_id, anti_cheat, &agent_id, &server_base, &allowed);

        if !anti_cheat {
            println!("[Agent] 防切屏未启用，仅记录考试状态");
            return;
        }

        let (cancel_tx, cancel_rx) = tokio::sync::oneshot::channel::<()>();
        self.cancel = Some(cancel_tx);

        tokio::spawn(async move {
            violation_detect_loop(cancel_rx, exam_id, agent_id, server_base, allowed).await;
        });
    }

    fn stop_sync(&mut self) {
        if let Some(tx) = self.cancel.take() {
            let _ = tx.send(());
            println!("[Agent] 考试模式已停止: exam_id={}", self.exam_id);
        }
        self.exam_id = 0;
        self.anti_cheat = false;
        self.allowed_window.clear();
        // 清除持久化状态
        std::fs::remove_file(exam_state_file()).ok();
    }
}

/// 保存考试状态到文件（断线恢复）
fn save_exam_state(exam_id: u64, anti_cheat: bool, agent_id: &str, server_base: &str, allowed_window: &str) {
    let state = json!({
        "exam_id": exam_id,
        "anti_cheat": anti_cheat,
        "agent_id": agent_id,
        "server_base": server_base,
        "allowed_window": allowed_window,
    });
    if let Ok(data) = serde_json::to_string(&state) {
        std::fs::write(exam_state_file(), data).ok();
    }
}

/// 从文件恢复考试状态（重连后调用）
fn load_exam_state() -> Option<(u64, bool, String, String, String)> {
    let data = std::fs::read_to_string(exam_state_file()).ok()?;
    let v: Value = serde_json::from_str(&data).ok()?;
    let exam_id = v.get("exam_id")?.as_u64()?;
    let anti_cheat = v.get("anti_cheat").and_then(|x| x.as_bool()).unwrap_or(true);
    let agent_id = v.get("agent_id").and_then(|x| x.as_str()).unwrap_or("").to_string();
    let server_base = v.get("server_base").and_then(|x| x.as_str()).unwrap_or("").to_string();
    let allowed_window = v.get("allowed_window").and_then(|x| x.as_str()).unwrap_or("").to_string();
    if exam_id == 0 { return None; }
    Some((exam_id, anti_cheat, agent_id, server_base, allowed_window))
}

/// 违规检测循环：每 500ms 检查活动窗口是否切换（增强版：切回重试+节流）
async fn violation_detect_loop(
    mut cancel_rx: tokio::sync::oneshot::Receiver<()>,
    exam_id: u64,
    agent_id: String,
    server_base: String,
    allowed_window: String,
) {
    let mut tick = interval(Duration::from_millis(500));
    let client = reqwest::Client::new();
    let mut violation_count = 0u32;
    // 违规节流：同一违规窗口 5 秒内不重复上报
    let mut last_violation_window = String::new();
    let mut last_violation_ts: i64 = 0;

    loop {
        tokio::select! {
            _ = &mut cancel_rx => {
                println!("[Agent] 违规检测循环收到取消信号");
                break;
            }
            _ = tick.tick() => {
                let current = platform::get_active_window_sync();
                if current.is_empty() || current == allowed_window {
                    continue; // 窗口未变化
                }

                // 检测到窗口切换 → 违规
                violation_count += 1;
                let win_name = platform::get_window_name_sync(&current);
                let detail = format!(
                    "第{}次切屏: 窗口从 {} 切换到 {} ({})",
                    violation_count, allowed_window, current, win_name
                );
                println!("[Agent] ⚠️ 考试违规: {}", detail);

                // 尝试切回允许窗口（最多重试 2 次）
                let mut restored = false;
                for attempt in 0..3 {
                    if platform::activate_window_sync(&allowed_window) {
                        // 等待 100ms 后验证是否切回
                        tokio::time::sleep(Duration::from_millis(100)).await;
                        if platform::get_active_window_sync() == allowed_window {
                            restored = true;
                            break;
                        }
                    }
                    if attempt < 2 {
                        tokio::time::sleep(Duration::from_millis(200)).await;
                    }
                }
                if !restored {
                    println!("[Agent] ⚠️ 切回窗口失败，允许窗口: {}", allowed_window);
                }

                // 违规节流：同一窗口 5 秒内不重复上报
                let now = now_ts();
                let should_report = current != last_violation_window || (now - last_violation_ts) >= 5;
                if should_report {
                    last_violation_window = current.clone();
                    last_violation_ts = now;

                    // 上报违规（带重试）
                    let url = format!("{}/api/v1/exam/violation", server_base);
                    let body = json!({
                        "exam_id": exam_id,
                        "agent_id": agent_id,
                        "type": "window_switch",
                        "detail": detail,
                    });
                    let mut reported = false;
                    for retry in 0..3 {
                        match client.post(&url).json(&body).send().await {
                            Ok(_) => { reported = true; break; }
                            Err(e) => {
                                println!("[Agent] 违规上报失败(第{}次): {}", retry + 1, e);
                                if retry < 2 {
                                    tokio::time::sleep(Duration::from_millis(500)).await;
                                }
                            }
                        }
                    }
                    if !reported {
                        println!("[Agent] ⚠️ 违规上报最终失败，本地记录: {}", detail);
                    } else {
                        // P1: 违纪自动截图（source=violation，上传失败不阻塞检测循环）
                        match take_snapshot(&agent_id, "violation").await {
                            Ok(u) => println!("[Agent] 违纪截图已上传: {}", u),
                            Err(e) => println!("[Agent] 违纪截图失败(不影响上报): {}", e),
                        }
                    }
                }
            }
        }
    }
    println!("[Agent] 违规检测循环结束，共检测 {} 次违规", violation_count);
}

impl RemoteDesktop {
    fn new() -> Self {
        RemoteDesktop { cancel: None, session_id: 0 }
    }

    fn is_active(&self) -> bool {
        self.cancel.is_some()
    }

    /// 启动截屏循环，帧通过 frame_tx 发送到 WS 写泵
    fn start(&mut self, session_id: u64, fps: u32, quality: u32, frame_tx: mpsc::Sender<Vec<u8>>) {
        self.stop_sync();
        self.session_id = session_id;

        let (cancel_tx, cancel_rx) = tokio::sync::oneshot::channel::<()>();
        self.cancel = Some(cancel_tx);

        let interval_ms: u64 = if fps > 0 { 1000 / fps as u64 } else { 100 };
        println!("[Agent] 远程桌面启动: session={} fps={} quality={} interval={}ms",
            session_id, fps, quality, interval_ms);

        tokio::spawn(async move {
            capture_loop(cancel_rx, frame_tx, interval_ms, quality, false).await;
        });
    }

    fn stop_sync(&mut self) {
        if let Some(tx) = self.cancel.take() {
            let _ = tx.send(());
            println!("[Agent] 远程桌面已停止: session={}", self.session_id);
        }
        self.session_id = 0;
    }
}

impl ThumbCapture {
    fn new() -> Self {
        ThumbCapture { cancel: None }
    }

    fn is_active(&self) -> bool {
        self.cancel.is_some()
    }

    /// 启动监控墙缩略图采集（默认 1fps、q40、320px 宽，独立于远程桌面）
    fn start(&mut self, fps: u32, quality: u32, frame_tx: mpsc::Sender<Vec<u8>>) {
        self.stop_sync();
        let (cancel_tx, cancel_rx) = tokio::sync::oneshot::channel::<()>();
        self.cancel = Some(cancel_tx);
        let fps = if fps == 0 { 1 } else { fps };
        let quality = if quality == 0 { 40 } else { quality };
        let interval_ms = 1000 / fps as u64;
        println!("[Agent] 监控墙缩略图启动: fps={} quality={} interval={}ms", fps, quality, interval_ms);
        tokio::spawn(async move {
            capture_loop(cancel_rx, frame_tx, interval_ms, quality, true).await;
        });
    }

    fn stop_sync(&mut self) {
        if let Some(tx) = self.cancel.take() {
            let _ = tx.send(());
            println!("[Agent] 监控墙缩略图已停止");
        }
    }
}

/// 截屏循环：platform::capture_frame_jpeg → 发送到 channel（P3-4 平台抽象）
/// thumb=true 时为监控墙缩略图模式：320px 宽、低画质、帧头最高位=1（与远程桌面帧区分）
async fn capture_loop(
    mut cancel_rx: tokio::sync::oneshot::Receiver<()>,
    frame_tx: mpsc::Sender<Vec<u8>>,
    interval_ms: u64,
    quality: u32,
    thumb: bool,
) {
    let mut tick = interval(Duration::from_millis(interval_ms));
    let mut frame_count: u64 = 0;
    let mut err_logged = false;

    loop {
        tokio::select! {
            _ = &mut cancel_rx => {
                println!("[Agent] 截屏循环收到取消信号");
                break;
            }
            _ = tick.tick() => {
                match platform::capture_frame_jpeg(quality, thumb) {
                    Some(jpeg_data) => {
                        frame_count += 1;
                        // 构建帧：4字节头(缩略图模式最高位置1) + JPEG数据
                        let header: u32 = if thumb {
                            0x8000_0000 | (frame_count as u32 & 0x7FFF_FFFF)
                        } else {
                            frame_count as u32
                        };
                        let mut frame = Vec::with_capacity(4 + jpeg_data.len());
                        frame.extend_from_slice(&header.to_le_bytes());
                        frame.extend_from_slice(&jpeg_data);
                        if frame_tx.send(frame).await.is_err() {
                            println!("[Agent] 帧通道已关闭，停止截屏");
                            break;
                        }
                    }
                    None => {
                        if !err_logged {
                            println!("[Agent] 屏幕采集失败（无显示环境或采集组件不可用）");
                            err_logged = true;
                        }
                    }
                }
            }
        }
    }
    println!("[Agent] 截屏循环结束，共发送 {} 帧", frame_count);
}

#[tokio::main]
async fn main() {
    let cfg = Config::from_env();
    println!("[Agent] 启动 agent_id={} server={}", cfg.agent_id, cfg.server_url);

    loop {
        match run_session(&cfg).await {
            Ok(_) => println!("[Agent] 会话正常结束"),
            Err(e) => println!("[Agent] 连接错误: {}，5 秒后重连", e),
        }
        tokio::time::sleep(Duration::from_secs(5)).await;
    }
}

async fn run_session(cfg: &Config) -> Result<(), Box<dyn std::error::Error>> {
    let url = url::Url::parse(&cfg.server_url)?;
    let (ws_stream, _) = connect_async(url).await?;
    println!("[Agent] WebSocket 已连接");
    let (mut write, mut read) = ws_stream.split();

    let player = Arc::new(Mutex::new(PlayerState::new()));
    let remote = Arc::new(Mutex::new(RemoteDesktop::new()));
    let thumb = Arc::new(Mutex::new(ThumbCapture::new())); // P2-1 监控墙缩略图
    let exam_guard = Arc::new(Mutex::new(ExamGuard::new()));
    let lock_guard = Arc::new(Mutex::new(LockGuard::new())); // P0-1 屏幕锁定

    // 从 WS URL 推导 HTTP base URL（用于违规上报）
    let server_base = cfg.server_url
        .replace("ws://", "http://")
        .replace("wss://", "https://")
        .split("/agent/ws").next().unwrap_or("http://localhost:8090").to_string();

    // 屏幕帧通道：截屏循环 → WS 写泵
    let (frame_tx, mut frame_rx) = mpsc::channel::<Vec<u8>>(32);

    // 注册
    let mut reg_payload = collect_sysinfo();
    reg_payload["agent_id"] = json!(cfg.agent_id);
    let reg = Frame::new("register", "reg_1", reg_payload);
    write.send(Message::Text(serde_json::to_string(&reg)?)).await?;
    println!("[Agent] 已发送注册帧");

    // 断线恢复：检查是否有未结束的考试状态
    if let Some((exam_id, anti_cheat, saved_agent_id, saved_server_base, saved_allowed)) = load_exam_state() {
        println!("[Agent] 检测到未结束的考试状态，恢复防切屏: exam_id={}", exam_id);
        let mut eg = exam_guard.lock().await;
        eg.exam_id = exam_id;
        eg.anti_cheat = anti_cheat;
        eg.allowed_window = saved_allowed.clone();
        if anti_cheat {
            let (cancel_tx, cancel_rx) = tokio::sync::oneshot::channel::<()>();
            eg.cancel = Some(cancel_tx);
            let agent_id_clone = saved_agent_id.clone();
            let server_base_clone = saved_server_base.clone();
            let allowed_clone = saved_allowed.clone();
            tokio::spawn(async move {
                violation_detect_loop(cancel_rx, exam_id, agent_id_clone, server_base_clone, allowed_clone).await;
            });
            println!("[Agent] 防切屏检测已恢复");
        }
    }

    let mut hb = interval(Duration::from_secs(cfg.heartbeat_secs));
    let mut hb_count = 0u64;

    loop {
        tokio::select! {
            _ = hb.tick() => {
                hb_count += 1;
                let frame = Frame::new("heartbeat", &format!("hb_{}", hb_count),
                    json!({"agent_id": cfg.agent_id}));
                write.send(Message::Text(serde_json::to_string(&frame)?)).await?;
            }
            // 屏幕帧 → 以 binary WS 消息发送
            Some(frame_data) = frame_rx.recv() => {
                write.send(Message::Binary(frame_data)).await?;
            }
            msg = read.next() => {
                match msg {
                    Some(Ok(Message::Text(text))) => {
                        let frame: Frame = serde_json::from_str(&text)?;
                        if frame.ftype == "cmd" {
                            let action = frame.payload.get("action")
                                .and_then(|v| v.as_str()).unwrap_or("");

                            // 远程桌面指令需要特殊处理（访问 frame_tx / remote）
                            match action {
                                "remote_start" => {
                                    let params = frame.payload.get("params").cloned().unwrap_or(json!({}));
                                    let session_id = params.get("session_id").and_then(|v| v.as_u64()).unwrap_or(0);
                                    let fps = params.get("fps").and_then(|v| v.as_u64()).unwrap_or(10) as u32;
                                    let quality = params.get("quality").and_then(|v| v.as_u64()).unwrap_or(60) as u32;
                                    let mut rd = remote.lock().await;
                                    rd.start(session_id, fps, quality, frame_tx.clone());
                                    let ack = Frame::new("ack", &frame.msg_id, json!({
                                        "agent_id": cfg.agent_id,
                                        "action": "remote_start",
                                        "status": "ok",
                                        "session_id": session_id,
                                    }));
                                    write.send(Message::Text(serde_json::to_string(&ack)?)).await?;
                                }
                                "remote_stop" => {
                                    let mut rd = remote.lock().await;
                                    rd.stop_sync();
                                    let ack = Frame::new("ack", &frame.msg_id, json!({
                                        "agent_id": cfg.agent_id,
                                        "action": "remote_stop",
                                        "status": "ok",
                                    }));
                                    write.send(Message::Text(serde_json::to_string(&ack)?)).await?;
                                }
                                // P2-1 监控墙缩略图采集
                                "thumb_start" => {
                                    let params = frame.payload.get("params").cloned().unwrap_or(json!({}));
                                    let fps = params.get("fps").and_then(|v| v.as_u64()).unwrap_or(1) as u32;
                                    let quality = params.get("quality").and_then(|v| v.as_u64()).unwrap_or(40) as u32;
                                    let mut tc = thumb.lock().await;
                                    tc.start(fps, quality, frame_tx.clone());
                                    let ack = Frame::new("ack", &frame.msg_id, json!({
                                        "agent_id": cfg.agent_id,
                                        "action": "thumb_start",
                                        "status": "ok",
                                    }));
                                    write.send(Message::Text(serde_json::to_string(&ack)?)).await?;
                                }
                                "thumb_stop" => {
                                    let mut tc = thumb.lock().await;
                                    tc.stop_sync();
                                    let ack = Frame::new("ack", &frame.msg_id, json!({
                                        "agent_id": cfg.agent_id,
                                        "action": "thumb_stop",
                                        "status": "ok",
                                    }));
                                    write.send(Message::Text(serde_json::to_string(&ack)?)).await?;
                                }
                                "remote_input" => {
                                    let params = frame.payload.get("params").cloned().unwrap_or(json!({}));
                                    let status = platform::send_input(&params).await;
                                    // 输入回执（可选，减少延迟可省略）
                                    let ack = Frame::new("ack", &frame.msg_id, json!({
                                        "agent_id": cfg.agent_id,
                                        "action": "remote_input",
                                        "status": status,
                                    }));
                                    write.send(Message::Text(serde_json::to_string(&ack)?)).await?;
                                }
                                "exam_start" => {
                                    let params = frame.payload.get("params").cloned().unwrap_or(json!({}));
                                    let exam_id = params.get("exam_id").and_then(|v| v.as_u64()).unwrap_or(0);
                                    let anti_cheat = params.get("anti_cheat").and_then(|v| v.as_bool()).unwrap_or(true);
                                    let mut eg = exam_guard.lock().await;
                                    eg.start(exam_id, anti_cheat, cfg.agent_id.clone(), server_base.clone());
                                    let ack = Frame::new("ack", &frame.msg_id, json!({
                                        "agent_id": cfg.agent_id,
                                        "action": "exam_start",
                                        "status": "ok",
                                        "exam_id": exam_id,
                                        "anti_cheat": anti_cheat,
                                    }));
                                    write.send(Message::Text(serde_json::to_string(&ack)?)).await?;
                                }
                                "exam_end" => {
                                    let mut eg = exam_guard.lock().await;
                                    eg.stop_sync();
                                    let ack = Frame::new("ack", &frame.msg_id, json!({
                                        "agent_id": cfg.agent_id,
                                        "action": "exam_end",
                                        "status": "ok",
                                    }));
                                    write.send(Message::Text(serde_json::to_string(&ack)?)).await?;
                                }
                                _ => {
                                    // 其他指令走通用处理
                                    let ack = handle_command(&cfg.agent_id, &frame, &player, &lock_guard).await;
                                    write.send(Message::Text(serde_json::to_string(&ack)?)).await?;
                                }
                            }
                        }
                    }
                    Some(Ok(Message::Ping(data))) => {
                        write.send(Message::Pong(data)).await?;
                    }
                    Some(Ok(Message::Close(_))) => {
                        println!("[Agent] 服务端关闭连接");
                        player.lock().await.stop().await;
                        remote.lock().await.stop_sync();
                        exam_guard.lock().await.stop_sync();
                        return Ok(());
                    }
                    Some(Err(e)) => {
                        player.lock().await.stop().await;
                        remote.lock().await.stop_sync();
                        exam_guard.lock().await.stop_sync();
                        return Err(Box::new(e));
                    }
                    None => {
                        player.lock().await.stop().await;
                        remote.lock().await.stop_sync();
                        exam_guard.lock().await.stop_sync();
                        return Ok(());
                    }
                    _ => {}
                }
            }
        }
    }
}

// ============ P3-3: 指令插件框架（CommandRegistry） ============
//
// 设计：每条指令在 COMMAND_REGISTRY 登记元数据（名称/说明/分类/危险级）。
// 新增指令 = ① 在注册表加一行 ② 在 handle_command（或 WS 媒体分支）实现处理。
// 服务器可下发 action="commands" 查询本机 Agent 支持的全部指令（能力自省），
// 便于按版本/平台动态渲染管理台按钮，实现"插件式"扩展而无需改协议。

/// 指令元数据定义
struct CommandDef {
    name: &'static str,
    description: &'static str,
    category: &'static str,
    dangerous: bool,
}

/// 全局指令注册表
static COMMAND_REGISTRY: &[CommandDef] = &[
    // ---- 屏幕/锁定 ----
    CommandDef { name: "lock", description: "锁屏：全屏覆盖层 + 键鼠抢占，Agent 退出自动解除", category: "屏幕", dangerous: false },
    CommandDef { name: "unlock", description: "解除锁屏", category: "屏幕", dangerous: false },
    CommandDef { name: "snapshot", description: "屏幕截图并上传服务端截图库", category: "屏幕", dangerous: false },
    // ---- 远程桌面/监控墙（媒体通道） ----
    CommandDef { name: "remote_start", description: "启动远程桌面帧采集", category: "远程", dangerous: false },
    CommandDef { name: "remote_stop", description: "停止远程桌面", category: "远程", dangerous: false },
    CommandDef { name: "remote_input", description: "远程键鼠输入注入", category: "远程", dangerous: false },
    CommandDef { name: "thumb_start", description: "启动监控墙缩略图采集（320px/1fps）", category: "远程", dangerous: false },
    CommandDef { name: "thumb_stop", description: "停止监控墙缩略图采集", category: "远程", dangerous: false },
    // ---- 电源 ----
    CommandDef { name: "shutdown", description: "延时关机（delay_sec，默认 3s）", category: "电源", dangerous: true },
    CommandDef { name: "reboot", description: "延时重启（delay_sec，默认 3s）", category: "电源", dangerous: true },
    // ---- 会话 ----
    CommandDef { name: "session_logoff", description: "注销/终止用户会话", category: "会话", dangerous: false },
    CommandDef { name: "exam_enter", description: "进入考试模式", category: "会话", dangerous: false },
    CommandDef { name: "exam_exit", description: "退出考试模式", category: "会话", dangerous: false },
    // ---- 消息/应用 ----
    CommandDef { name: "message", description: "弹窗消息（notify/confirm），支持已读回执", category: "消息", dangerous: false },
    CommandDef { name: "open", description: "启动应用或打开网址", category: "应用", dangerous: false },
    CommandDef { name: "exec", description: "执行命令（服务端+本机双重白名单）", category: "应用", dangerous: true },
    // ---- 文件 ----
    CommandDef { name: "file_recv", description: "接收下发文件并回报进度", category: "文件", dangerous: false },
    CommandDef { name: "collect_files", description: "按 pattern 收集学生作业并上传", category: "文件", dangerous: false },
    // ---- 广播 ----
    CommandDef { name: "broadcast_join", description: "加入屏幕广播（组播）", category: "广播", dangerous: false },
    CommandDef { name: "broadcast_stop", description: "退出屏幕广播", category: "广播", dangerous: false },
    // ---- 运维 ----
    CommandDef { name: "agent_upgrade", description: "Agent 自升级（下载→原子替换→重启）", category: "运维", dangerous: true },
    CommandDef { name: "commands", description: "能力自省：返回本注册表", category: "运维", dangerous: false },
];

/// 注册表 → JSON（供 commands 指令与启动自检）
fn registry_json() -> Value {
    json!(COMMAND_REGISTRY.iter().map(|d| json!({
        "name": d.name,
        "description": d.description,
        "category": d.category,
        "dangerous": d.dangerous,
    })).collect::<Vec<_>>())
}

#[cfg(test)]
mod registry_tests {
    use super::COMMAND_REGISTRY;
    use std::collections::HashSet;

    #[test]
    fn registry_names_unique() {
        let mut seen = HashSet::new();
        for d in COMMAND_REGISTRY {
            assert!(seen.insert(d.name), "注册表指令名重复: {}", d.name);
        }
    }

    #[test]
    fn registry_nonempty() {
        assert!(COMMAND_REGISTRY.len() >= 20);
    }
}

/// 处理通用控制指令，返回 ack 帧（P0-1: 真实执行）
async fn handle_command(agent_id: &str, frame: &Frame, player: &Arc<Mutex<PlayerState>>, lock_guard: &Arc<Mutex<LockGuard>>) -> Frame {
    let action = frame.payload.get("action")
        .and_then(|v| v.as_str())
        .unwrap_or("unknown");
    let params = frame.payload.get("params").cloned().unwrap_or(json!({}));

    println!("[Agent] 收到指令: {} 参数: {}", action, params);

    let mut detail = String::new();
    let status = match action {
        // ---- 屏幕锁定：X11 全屏覆盖层 + 键鼠抢占，Agent 退出自动解除 ----
        "lock" => {
            let message = params.get("message").and_then(|v| v.as_str()).unwrap_or("").to_string();
            let mut lg = lock_guard.lock().await;
            if lg.is_locked() {
                detail = "已处于锁定状态".to_string();
                "ok"
            } else if let Err(e) = platform::display_available() {
                // 预检失败：无头环境/无 DISPLAY → 如实报 error，不假装锁定成功
                detail = format!("锁屏失败: {}", e);
                "error"
            } else {
                lg.start(message);
                detail = "屏幕已锁定（全屏覆盖层已启动）".to_string();
                "ok"
            }
        }
        "unlock" => {
            let mut lg = lock_guard.lock().await;
            if lg.is_locked() {
                lg.stop();
                detail = "屏幕锁定已解除".to_string();
            } else {
                detail = "当前未锁定".to_string();
            }
            "ok"
        }
        // ---- 关机/重启：先回 ack，delay 秒后真实执行（让 ack 有时间发出） ----
        "shutdown" | "reboot" => {
            let is_reboot = action == "reboot";
            let delay = params.get("delay_sec").and_then(|v| v.as_u64()).unwrap_or(3).min(60);
            tokio::spawn(async move {
                tokio::time::sleep(std::time::Duration::from_secs(delay)).await;
                platform::execute_power(is_reboot);
            });
            detail = format!("{}秒后执行{}", delay, if is_reboot { "重启" } else { "关机" });
            "ok"
        }
        "broadcast_join" => {
            let mcast = params.get("mcast_addr").and_then(|v| v.as_str()).unwrap_or("");
            if mcast.is_empty() {
                "error_no_addr"
            } else {
                let mut p = player.lock().await;
                match p.start(mcast).await {
                    Ok(s) => {
                        return Frame::new("ack", &frame.msg_id, json!({
                            "agent_id": agent_id, "action": action,
                            "status": s, "mcast_addr": mcast,
                        }));
                    }
                    Err(e) => {
                        return Frame::new("ack", &frame.msg_id, json!({
                            "agent_id": agent_id, "action": action,
                            "status": "error", "error": e,
                        }));
                    }
                }
            }
        }
        "broadcast_stop" => {
            let mut p = player.lock().await;
            p.stop().await;
            "ok"
        }
        "file_recv" => {
            let download_url = params.get("download_url").and_then(|v| v.as_str()).unwrap_or("");
            let filename = params.get("filename").and_then(|v| v.as_str()).unwrap_or("unknown");
            let task_id = params.get("task_id").and_then(|v| v.as_u64()).unwrap_or(0);
            let file_size = params.get("size").and_then(|v| v.as_u64()).unwrap_or(0);
            if download_url.is_empty() {
                "error_no_url"
            } else {
                println!("  -> 下载文件: {} ({} bytes)", filename, file_size);
                let agent_id_owned = agent_id.to_string();
                let url_owned = download_url.to_string();
                let fname_owned = filename.to_string();
                tokio::spawn(async move {
                    download_and_report(&agent_id_owned, &url_owned, &fname_owned, task_id, file_size).await;
                });
                "downloading"
            }
        }
        "exam_enter" => { println!("  -> 进入考试模式"); "ok" }
        "exam_exit" => { println!("  -> 退出考试模式"); "ok" }
        // ---- P1: 消息弹窗（notify 定时消失 / confirm 需学生按键，已读回执 HTTP 上报） ----
        "message" => {
            let msg_id = params.get("message_id").and_then(|v| v.as_u64()).unwrap_or(0);
            let title = params.get("title").and_then(|v| v.as_str()).unwrap_or("教师消息").to_string();
            let body = params.get("body").and_then(|v| v.as_str()).unwrap_or("").to_string();
            let mode = params.get("mode").and_then(|v| v.as_str()).unwrap_or("notify").to_string();
            let duration = params.get("duration").and_then(|v| v.as_u64()).unwrap_or(15).max(3).min(600);
            if let Err(e) = platform::display_available() {
                detail = format!("弹窗失败: {}", e);
                "error"
            } else {
                let agent_id_owned = agent_id.to_string();
                let server = server_base_from_env();
                detail = format!("消息已显示（{} 模式，{}秒）", mode, duration);
                tokio::spawn(async move {
                    let t = title; let b = body; let m = mode;
                    let read = tokio::task::spawn_blocking(move || platform::message_overlay_sync(&t, &b, &m, duration))
                        .await.unwrap_or(false);
                    if read && msg_id > 0 && !server.is_empty() {
                        let url = format!("{}/api/v1/msg/read", server);
                        let client = reqwest::Client::new();
                        for retry in 0..3 {
                            match client.post(&url).json(&json!({
                                "message_id": msg_id, "agent_id": agent_id_owned,
                            })).send().await {
                                Ok(_) => break,
                                Err(e) => {
                                    println!("[Agent] 已读回执上报失败(第{}次): {}", retry + 1, e);
                                    tokio::time::sleep(Duration::from_millis(500)).await;
                                }
                            }
                        }
                    }
                });
                "ok"
            }
        }
        // ---- P1: 屏幕截图（scrot 抓屏 → JPEG → multipart 上传服务端） ----
        "snapshot" => {
            let source = params.get("source").and_then(|v| v.as_str()).unwrap_or("manual").to_string();
            match take_snapshot(agent_id, &source).await {
                Ok(url) => { detail = format!("截图已上传: {}", url); "ok" }
                Err(e) => { detail = format!("截图失败: {}", e); "error" }
            }
        }
        // ---- P1: 桌面服务（app 直接启动 / url 用 xdg-open 打开） ----
        "open" => {
            let typ = params.get("type").and_then(|v| v.as_str()).unwrap_or("app");
            let target = params.get("target").and_then(|v| v.as_str()).unwrap_or("").trim().to_string();
            if target.is_empty() {
                detail = "缺少 target 参数".to_string();
                "error"
            } else {
                match platform::open_target(typ, &target) {
                    Ok(d) => { detail = d; "ok" }
                    Err(e) => { detail = e; "error" }
                }
            }
        }
        // ---- exec：白名单二次校验 + 真实执行 + 输出回报（60s 超时） ----
        "exec" => {
            let cmd = params.get("cmd").and_then(|v| v.as_str()).unwrap_or("").trim().to_string();
            if cmd.is_empty() {
                detail = "缺少 cmd 参数".to_string();
                "error"
            } else if !agent_exec_allowed(&cmd) {
                println!("  -> 命令被本机白名单拒绝: {}", cmd);
                detail = "命令不在本机执行白名单(LAB_EXEC_WHITELIST)中".to_string();
                "rejected"
            } else {
                match run_exec(&cmd).await {
                    Ok(out) => { detail = out; "ok" }
                    Err(e) => { detail = e; "error" }
                }
            }
        }
        // ---- Agent 自升级：下载新二进制 → 原子替换 → 退出等守护进程拉起 ----
        "agent_upgrade" => {
            let url = params.get("download_url").and_then(|v| v.as_str()).unwrap_or("");
            if url.is_empty() {
                detail = "缺少 download_url 参数".to_string();
                "error_no_url"
            } else {
                match do_self_upgrade(url).await {
                    Ok(msg) => {
                        detail = msg;
                        // 让 ack 先发出，2 秒后退出（systemd Restart=always 拉起新版本）
                        tokio::spawn(async {
                            tokio::time::sleep(std::time::Duration::from_secs(2)).await;
                            println!("[Agent] 升级完成，退出等待守护进程重启");
                            std::process::exit(0);
                        });
                        "ok"
                    }
                    Err(e) => { detail = e; "error" }
                }
            }
        }
        // ---- P2-4: 会话控制 —— 注销/终止用户会话（loginctl） ----
        "session_logoff" => {
            let user = params.get("user").and_then(|v| v.as_str()).unwrap_or("").trim().to_string();
            let mode = params.get("mode").and_then(|v| v.as_str()).unwrap_or("logoff"); // logoff | kill
            // 平台层：Linux loginctl / Windows WTSQueryUserToken+winlogoff → ExitWindowsEx
            match platform::session_logoff(&user, mode).await {
                Ok(d) => { detail = d; "ok" }
                Err(e) => { detail = e; "error" }
            }
        }
        // ---- P2-5: 作业收集 —— 按 pattern 找文件并逐个上传服务端 ----
        "collect_files" => {
            let task_id = params.get("task_id").and_then(|v| v.as_u64()).unwrap_or(0);
            let pattern = params.get("pattern").and_then(|v| v.as_str()).unwrap_or("").trim().to_string();
            if task_id == 0 || pattern.is_empty() {
                detail = "缺少 task_id/pattern".to_string();
                "error"
            } else {
                match collect_and_upload(agent_id, task_id, &pattern).await {
                    Ok(n) => { detail = format!("已上传 {} 个文件", n); "ok" }
                    Err(e) => { detail = e; "error" }
                }
            }
        }
        // ---- P3-3: 能力自省 —— 返回指令注册表（含平台标识） ----
        "commands" => {
            let (os, ver) = detect_os();
            return Frame::new("ack", &frame.msg_id, json!({
                "agent_id": agent_id,
                "action": action,
                "status": "ok",
                "detail": format!("{} 条指令", COMMAND_REGISTRY.len()),
                "commands": registry_json(),
                "platform": json!({ "os": os, "version": ver, "agent_version": env!("CARGO_PKG_VERSION") }),
            }));
        }
        _ => { println!("  -> 未知指令: {}", action); "unknown_action" }
    };

    Frame::new("ack", &frame.msg_id, json!({
        "agent_id": agent_id,
        "action": action,
        "status": status,
        "detail": detail,
    }))
}

// ============ P0-1: 指令执行辅助函数 ============

/// Agent 侧 exec 白名单（第二道防线，服务端已先校验一次）。
/// LAB_EXEC_WHITELIST 换行分隔命令前缀；未设置/为空时信任服务端校验结果。
fn agent_exec_allowed(cmd: &str) -> bool {
    let v = match std::env::var("LAB_EXEC_WHITELIST") {
        Ok(v) if v.trim().is_empty() => return true,
        Ok(v) => v,
        Err(_) => return true,
    };
    for entry in v.split('\n') {
        let entry = entry.trim();
        if entry.is_empty() { continue; }
        if cmd == entry || cmd.starts_with(&format!("{} ", entry)) {
            return true;
        }
    }
    false
}

/// 真实执行命令（Linux sh -c / Windows cmd /C），60 秒超时，回报 exit code + stdout/stderr（截断 900 字符防撑爆 ack）
async fn run_exec(cmd: &str) -> Result<String, String> {
    println!("[Agent] 执行命令: {}", cmd);
    let fut = platform::run_shell(cmd);
    let out = tokio::time::timeout(std::time::Duration::from_secs(60), fut)
        .await
        .map_err(|_| "执行超时(60s)".to_string())?
        .map_err(|e| format!("启动失败: {}", e))?;
    let code = out.status.code().unwrap_or(-1);
    let stdout = String::from_utf8_lossy(&out.stdout);
    let stderr = String::from_utf8_lossy(&out.stderr);
    let mut s = format!("exit={}", code);
    if !stdout.trim().is_empty() {
        s.push_str("\n[stdout] ");
        s.push_str(stdout.trim());
    }
    if !stderr.trim().is_empty() {
        s.push_str("\n[stderr] ");
        s.push_str(stderr.trim());
    }
    if s.len() > 900 {
        s.truncate(900);
        s.push_str("…(已截断)");
    }
    Ok(s)
}

// ============ P1: 消息弹窗 / 截图 辅助函数 ============

/// 从 LAB_SERVER 推导服务端 HTTP 基址（与 violation_detect_loop 的 server_base 同规则）
fn server_base_from_env() -> String {
    std::env::var("LAB_SERVER")
        .unwrap_or_else(|_| "ws://127.0.0.1:8090/agent/ws".to_string())
        .replace("ws://", "http://")
        .replace("wss://", "https://")
        .split("/agent/ws").next().unwrap_or("http://localhost:8090").to_string()
}

/// 屏幕截图：scrot 抓屏 → convert 转 JPEG → multipart 上传服务端 → 返回访问 URL
async fn take_snapshot(agent_id: &str, source: &str) -> Result<String, String> {
    // 1) 抓屏 + JPEG 编码（平台层：Linux scrot/convert，Windows DXGI/GDI）
    let jpeg = platform::capture_screen_jpeg()?;
    // 3) multipart 上传（每次重试重建 Form，reqwest Form 不可 clone）
    let server = server_base_from_env();
    let url = format!("{}/api/v1/screenshots/upload", server);
    let fname = format!("{}_{}.jpg", agent_id, chrono_stamp());
    let client = reqwest::Client::new();
    let mut last_err = "上传失败".to_string();
    for retry in 0..3 {
        let part = reqwest::multipart::Part::bytes(jpeg.clone())
            .file_name(fname.clone())
            .mime_str("image/jpeg").map_err(|e| e.to_string())?;
        let form = reqwest::multipart::Form::new()
            .part("file", part)
            .text("agent_id", agent_id.to_string())
            .text("source", source.to_string());
        match client.post(&url).multipart(form).send().await {
            Ok(resp) => {
                let v: serde_json::Value = resp.json().await.unwrap_or(json!({}));
                if let Some(u) = v.get("data").and_then(|d| d.get("url")).and_then(|u| u.as_str()) {
                    return Ok(u.to_string());
                }
                last_err = format!("服务端响应异常: {}", v);
            }
            Err(e) => last_err = e.to_string(),
        }
        if retry < 2 { tokio::time::sleep(Duration::from_millis(500)).await; }
    }
    Err(last_err)
}

fn chrono_stamp() -> String {
    let now = std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH).unwrap_or_default();
    format!("{}", now.as_secs())
}

/// P2-5: 按 pattern 匹配文件并 multipart 上传到服务端 /collect/upload。
/// pattern 格式：`~/Desktop/*.docx`（目录 + 文件名通配）。支持 `*` 与 `?`，
/// 也兼容 `~` 展开与相对路径（相对 agent 工作目录）。
/// 返回成功上传的文件数。
async fn collect_and_upload(agent_id: &str, task_id: u64, pattern: &str) -> Result<usize, String> {
    // 平台层 pattern 归一化（~ 展开、Windows 反斜杠）
    let (dir, file_pat) = platform::normalize_pattern(pattern)?;
    // 通配 → 正则式匹配
    fn matches(pat: &str, name: &str) -> bool {
        let p = pat.chars().peekable();
        fn rec(mut p: std::iter::Peekable<std::str::Chars>, mut s: std::iter::Peekable<std::str::Chars>) -> bool {
            while let Some(&pc) = p.peek() {
                match pc {
                    '*' => {
                        p.next();
                        if p.peek().is_none() { return true; }
                        while s.peek().is_some() {
                            if rec(p.clone(), s.clone()) { return true; }
                            s.next();
                        }
                        return rec(p, s);
                    }
                    '?' => { p.next(); if s.next().is_none() { return false; } }
                    c => {
                        p.next();
                        match s.next() {
                            Some(sc) if sc == c => {}
                            _ => return false,
                        }
                    }
                }
            }
            s.peek().is_none()
        }
        rec(p, name.chars().peekable())
    }
    let mut entries = std::fs::read_dir(&dir)
        .map_err(|e| format!("读取目录失败 {}: {}", dir, e))?;
    let mut files: Vec<std::path::PathBuf> = Vec::new();
    while let Some(Ok(entry)) = entries.next() {
        let path = entry.path();
        if path.is_file() {
            if let Some(name) = path.file_name().and_then(|n| n.to_str()) {
                if matches(&file_pat, name) {
                    files.push(path);
                }
            }
        }
    }
    if files.is_empty() {
        return Err(format!("未匹配到文件: {}", pattern));
    }
    let server = server_base_from_env();
    let url = format!("{}/api/v1/collect/upload", server);
    let client = reqwest::Client::new();
    let mut uploaded = 0usize;
    let mut first_fail: Option<String> = None;
    for path in &files {
        let bytes = match std::fs::read(path) {
            Ok(b) => b,
            Err(_) => continue,
        };
        let fname = path.file_name().unwrap_or_default().to_string_lossy().to_string();
        let mut last_err = String::new();
        let mut file_ok = false;
        for retry in 0..3 {
            let part = match reqwest::multipart::Part::bytes(bytes.clone())
                .file_name(fname.clone())
                .mime_str("application/octet-stream") {
                Ok(p) => p,
                Err(e) => { last_err = e.to_string(); break; }
            };
            let form = reqwest::multipart::Form::new()
                .part("file", part)
                .text("task_id", task_id.to_string())
                .text("agent_id", agent_id.to_string());
            match client.post(&url).multipart(form).send().await {
                Ok(resp) if resp.status().is_success() => { file_ok = true; break; }
                Ok(resp) => { last_err = format!("HTTP {}", resp.status()); }
                Err(e) => { last_err = e.to_string(); }
            }
            if retry < 2 { tokio::time::sleep(Duration::from_millis(300)).await; }
        }
        if file_ok {
            uploaded += 1;
        } else if first_fail.is_none() {
            first_fail = Some(format!("{}: {}", fname, last_err));
        }
    }
    if uploaded == 0 {
        return Err(first_fail.unwrap_or_else(|| "所有文件上传失败".to_string()));
    }
    Ok(uploaded)
}

/// Agent 自升级：下载新二进制 → chmod 755 → rename 原子替换当前 exe。
/// 返回后由调用方延迟退出，交给 systemd/supervisor 拉起新版本。
async fn do_self_upgrade(url: &str) -> Result<String, String> {
    let exe = std::env::current_exe().map_err(|e| format!("获取当前路径失败: {}", e))?;
    let exe_str = exe.to_string_lossy().to_string();
    let tmp = format!("{}.new", exe_str);

    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(300))
        .build()
        .map_err(|e| e.to_string())?;
    let bytes = client.get(url).send().await
        .map_err(|e| format!("下载失败: {}", e))?
        .error_for_status()
        .map_err(|e| format!("下载失败: HTTP {:?}", e.status()))?
        .bytes().await
        .map_err(|e| format!("读取失败: {}", e))?;
    if bytes.len() < 1024 {
        return Err(format!("下载内容异常过小({} bytes)，已中止", bytes.len()));
    }
    std::fs::write(&tmp, &bytes).map_err(|e| format!("写入临时文件失败: {}", e))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        std::fs::set_permissions(&tmp, std::fs::Permissions::from_mode(0o755))
            .map_err(|e| format!("设置权限失败: {}", e))?;
    }
    // 快速完整性校验：新文件必须是有效可执行文件（Linux ELF / Windows MZ）
    if !platform::is_exec_magic(&bytes) {
        std::fs::remove_file(&tmp).ok();
        return Err("下载文件不是有效可执行二进制，已中止".to_string());
    }
    platform::replace_running_exe(&tmp, &exe_str)?;
    println!("[Agent] 二进制已替换: {} ({} bytes)", exe_str, bytes.len());
    Ok(format!("已替换二进制({} bytes)，即将退出由守护进程重启", bytes.len()))
}

/// 异步下载文件并上报进度
async fn download_and_report(agent_id: &str, url: &str, filename: &str, task_id: u64, total_size: u64) {
    let recv_dir = std::env::var("LAB_RECV_DIR").unwrap_or_else(|_| platform::default_recv_dir());
    std::fs::create_dir_all(&recv_dir).ok();
    let save_path = format!("{}/{}", recv_dir, filename);
    let base_url = url.split("/api/v1/").next().unwrap_or("http://localhost:8090");
    let client = reqwest::Client::new();

    report_progress(&client, base_url, task_id, agent_id, "transferring", 0, "").await;

    let resp = match client.get(url).send().await {
        Ok(r) => r,
        Err(e) => {
            report_progress(&client, base_url, task_id, agent_id, "failed", 0, &e.to_string()).await;
            return;
        }
    };
    if !resp.status().is_success() {
        let err_msg = format!("HTTP {}", resp.status());
        report_progress(&client, base_url, task_id, agent_id, "failed", 0, &err_msg).await;
        return;
    }

    let mut stream = resp.bytes_stream();
    let mut file = match std::fs::File::create(&save_path) {
        Ok(f) => f,
        Err(e) => {
            report_progress(&client, base_url, task_id, agent_id, "failed", 0, &e.to_string()).await;
            return;
        }
    };

    let mut downloaded: u64 = 0;
    let mut last_percent = 0u32;
    while let Some(chunk) = stream.next().await {
        match chunk {
            Ok(bytes) => {
                if let Err(e) = file.write_all(&bytes) {
                    report_progress(&client, base_url, task_id, agent_id, "failed", 0, &e.to_string()).await;
                    return;
                }
                downloaded += bytes.len() as u64;
                if total_size > 0 {
                    let percent = ((downloaded * 100) / total_size) as u32;
                    if percent >= last_percent + 10 || percent >= 100 {
                        last_percent = percent;
                        report_progress(&client, base_url, task_id, agent_id, "transferring", percent, "").await;
                    }
                }
            }
            Err(e) => {
                report_progress(&client, base_url, task_id, agent_id, "failed", 0, &e.to_string()).await;
                return;
            }
        }
    }
    file.flush().ok();
    println!("  -> 文件下载完成: {} ({} bytes)", save_path, downloaded);
    report_progress(&client, base_url, task_id, agent_id, "done", 100, "").await;
}

async fn report_progress(client: &reqwest::Client, base_url: &str, task_id: u64, agent_id: &str, status: &str, percent: u32, error: &str) {
    let url = format!("{}/api/v1/files/progress", base_url);
    let body = json!({
        "task_id": task_id, "agent_id": agent_id,
        "status": status, "percent": percent, "error": error,
    });
    if let Err(e) = client.post(&url).json(&body).send().await {
        println!("  -> 进度上报失败: {}", e);
    }
}
