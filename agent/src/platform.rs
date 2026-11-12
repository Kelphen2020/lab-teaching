// ============ P3-4: 平台抽象层（Linux / Windows） ============
//
// main.rs 只调用本模块的统一接口；平台差异全部收敛在这里：
//   - Linux：X11（x11-dl 运行时加载 libX11）+ zenity/scrot/convert/xdotool/loginctl
//   - Windows：Win32 API。除 kernel32（std 必然链接）外，user32/gdi32/gdiplus/
//     d3d11/dxgi/advapi32/wtsapi32 全部通过 LoadLibraryW + GetProcAddress
//     【运行时】加载 —— 零编译期链接依赖，交叉编译只需 rust 目标，无需 mingw 链接器。
//
// 屏幕采集：DXGI Desktop Duplication（主） → GDI BitBlt（备） → GDI+ JPEG / BMP 兜底。
// 输入注入：SendInput；锁屏：WS_POPUP 全屏置顶窗 + 低级钩子抢占键鼠；
// 关机/重启：InitiateSystemShutdownExW（提权 SE_SHUTDOWN_NAME），回退 shutdown.exe。

// ---------------- 跨平台路径/环境 ----------------

/// 临时目录（Linux=/tmp，Windows=%TEMP%）
pub fn tmp_dir() -> String {
    std::env::temp_dir().to_string_lossy().to_string()
}

/// HOME（Linux）/ USERPROFILE（Windows）
pub fn home_dir() -> String {
    std::env::var("HOME")
        .or_else(|_| std::env::var("USERPROFILE"))
        .unwrap_or_else(|_| {
            if cfg!(windows) { r"C:\Users\Default".to_string() } else { "/root".to_string() }
        })
}

/// 文件接收默认目录（可被 LAB_RECV_DIR 覆盖）
pub fn default_recv_dir() -> String {
    format!("{}/lab-recv", tmp_dir())
}

/// 作业收集 pattern 归一化：'~' 展开为用户目录，返回 (目录, 文件名通配)
pub fn normalize_pattern(pattern: &str) -> Result<(String, String), String> {
    let sep = if cfg!(windows) { '\\' } else { '/' };
    let mut p: String = if pattern.starts_with('~') {
        format!("{}{}", home_dir(), &pattern[1..])
    } else {
        pattern.to_string()
    };
    if cfg!(windows) {
        p = p.replace('/', "\\");
    }
    let (dir, file_pat) = match p.rfind(sep) {
        Some(pos) => (p[..pos].to_string(), p[pos + 1..].to_string()),
        None => (".".to_string(), p.clone()),
    };
    if file_pat.is_empty() {
        return Err("pattern 缺少文件名部分".to_string());
    }
    Ok((dir, file_pat))
}

/// 可执行文件魔数校验（Linux=ELF，Windows=MZ）
pub fn is_exec_magic(bytes: &[u8]) -> bool {
    if cfg!(windows) {
        bytes.len() >= 2 && &bytes[0..2] == b"MZ"
    } else {
        bytes.len() >= 4 && &bytes[0..4] == b"\x7fELF"
    }
}

// ---------------- 统一接口声明（实现见各平台 mod） ----------------
//
// display_available()        图形环境预检（lock/message/snapshot 先检后做）
// lock_overlay_main(rx,msg)  锁屏覆盖层（阻塞调用线程，rx 收到信号解除）
// message_overlay_sync(...)  消息弹窗，返回是否已读
// capture_screen_jpeg()      截图 → JPEG 字节（1280px q75）
// capture_frame_jpeg(q,thumb) 帧循环截图（缩略图 320px）
// send_input(params)         键鼠注入
// get_active_window_sync()   活动窗口 ID
// get_window_name_sync(id)   窗口标题
// activate_window_sync(id)   激活窗口
// execute_power(reboot)      真实关机/重启（LAB_POWER_DRYRUN=1 跳过）
// session_logoff(user,mode)  注销会话
// run_shell(cmd)             执行 shell（Linux sh -c / Windows cmd /C）
// open_target(typ,target)    打开应用/网址，返回详情
// replace_running_exe(tmp,exe) 自升级原子替换（Windows 先挪走运行中的 exe）

// ================================================================
//                        Linux 实现（原有逻辑迁移）
// ================================================================

#[cfg(unix)]
mod imp {
    use serde_json::Value;
    use tokio::process::Command;

    pub fn display_available() -> Result<(), String> {
        use x11_dl::xlib::Xlib;
        let xlib = Xlib::open().map_err(|e| format!("无法加载 libX11.so: {}", e))?;
        unsafe {
            let dpy = (xlib.XOpenDisplay)(std::ptr::null());
            if dpy.is_null() {
                return Err("无法连接 X server（DISPLAY 未设置？）".to_string());
            }
            (xlib.XCloseDisplay)(dpy);
        }
        Ok(())
    }

    pub fn lock_overlay_main(cancel_rx: std::sync::mpsc::Receiver<()>, message: String) {
        use x11_dl::xlib::*;
        let xlib = match Xlib::open() {
            Ok(x) => x,
            Err(e) => { println!("[Agent] 锁屏失败: 无法加载 libX11.so: {}", e); return; }
        };
        unsafe {
            let dpy = (xlib.XOpenDisplay)(std::ptr::null());
            if dpy.is_null() {
                println!("[Agent] 锁屏失败: 无法连接 X server（DISPLAY 未设置？）");
                return;
            }
            let screen = (xlib.XDefaultScreen)(dpy);
            let root = (xlib.XRootWindow)(dpy, screen);
            let w = (xlib.XDisplayWidth)(dpy, screen) as u32;
            let h = (xlib.XDisplayHeight)(dpy, screen) as u32;
            let black = (xlib.XBlackPixel)(dpy, screen);
            let white = (xlib.XWhitePixel)(dpy, screen);

            // 全屏黑窗口，override_redirect 绕过窗口管理器
            let win = (xlib.XCreateSimpleWindow)(dpy, root, 0, 0, w, h, 0, black, black);
            let mut attrs: XSetWindowAttributes = std::mem::zeroed();
            attrs.override_redirect = 1;
            (xlib.XChangeWindowAttributes)(dpy, win, CWOverrideRedirect, &mut attrs);
            (xlib.XSelectInput)(dpy, win, ExposureMask | StructureNotifyMask);
            (xlib.XMapRaised)(dpy, win);
            (xlib.XRaiseWindow)(dpy, win);

            // 文本（fixed 字体仅支持 ASCII，中文提示降级为英文）
            let gc = (xlib.XDefaultGC)(dpy, screen);
            let font = (xlib.XLoadFont)(dpy, b"fixed\0".as_ptr() as *const std::os::raw::c_char);
            (xlib.XSetFont)(dpy, gc, font);
            (xlib.XSetForeground)(dpy, gc, white);
            let title_raw = format!("SCREEN LOCKED - {}", message);
            let ascii: String = title_raw.chars().filter(|c| c.is_ascii()).collect();
            let ascii = if ascii.trim().is_empty() { "SCREEN LOCKED".to_string() } else { ascii };

            let conn_fd = (xlib.XConnectionNumber)(dpy);
            let max_secs: u64 = std::env::var("LAB_LOCK_MAX_MIN").ok()
                .and_then(|s| s.parse().ok()).unwrap_or(240u64) * 60;
            let start = std::time::Instant::now();
            let mut tick = std::time::Instant::now() - std::time::Duration::from_secs(5);

            loop {
                if cancel_rx.try_recv().is_ok() { break; }
                if start.elapsed().as_secs() > max_secs {
                    println!("[Agent] 锁屏超过 {} 分钟，看门狗自动解除", max_secs / 60);
                    break;
                }
                // 清空事件队列
                while (xlib.XPending)(dpy) > 0 {
                    let mut ev: XEvent = std::mem::zeroed();
                    (xlib.XNextEvent)(dpy, &mut ev);
                }
                // 每秒：重抢占键鼠 + 置顶 + 重绘
                if tick.elapsed().as_millis() >= 1000 {
                    (xlib.XGrabKeyboard)(dpy, win, 1, GrabModeAsync, GrabModeAsync, CurrentTime);
                    (xlib.XGrabPointer)(dpy, win, 1, 0xFFFF, GrabModeAsync, GrabModeAsync, win, 0, CurrentTime);
                    (xlib.XRaiseWindow)(dpy, win);
                    let cx = (w / 2) as i32 - (ascii.len() as i32) * 4;
                    let cy = (h / 2) as i32;
                    (xlib.XDrawString)(dpy, win, gc, cx, cy,
                        ascii.as_ptr() as *const std::os::raw::c_char, ascii.len() as i32);
                    (xlib.XFlush)(dpy);
                    tick = std::time::Instant::now();
                }
                // 200ms 超时等待 X 事件（兼作取消信号轮询节拍）
                let mut rfds: libc::fd_set = std::mem::zeroed();
                libc::FD_ZERO(&mut rfds);
                libc::FD_SET(conn_fd, &mut rfds);
                let mut tv = libc::timeval { tv_sec: 0, tv_usec: 200_000 };
                libc::select(conn_fd + 1, &mut rfds, std::ptr::null_mut(), std::ptr::null_mut(), &mut tv);
            }
            (xlib.XUngrabKeyboard)(dpy, CurrentTime);
            (xlib.XUngrabPointer)(dpy, CurrentTime);
            (xlib.XDestroyWindow)(dpy, win);
            (xlib.XFlush)(dpy);
            (xlib.XCloseDisplay)(dpy);
            println!("[Agent] 锁屏覆盖层已退出");
        }
    }

    pub fn message_overlay_sync(title: &str, body: &str, mode: &str, duration: u64) -> bool {
        println!("[Agent] 显示消息弹窗: [{}] {} (mode={}, {}s)", title, body, mode, duration);
        // 1) zenity 原生对话框（中文友好）
        if let Ok(path) = which_bin("zenity") {
            let mut cmd = std::process::Command::new(path);
            if mode == "confirm" {
                cmd.args(["--info", "--title", title, "--text", body, "--width=420", "--ok-label=我已阅读"]);
            } else {
                cmd.args(["--info", "--title", title, "--text", body, "--width=420",
                          "--timeout", &duration.to_string(), "--no-wrap"]);
            }
            match cmd.status() {
                Ok(st) => {
                    if mode == "confirm" { return st.success(); }
                    return true; // notify：窗口已显示过
                }
                Err(e) => println!("[Agent] zenity 启动失败: {}，降级 X11 覆盖层", e),
            }
        }
        // 2) X11 覆盖层降级
        message_overlay_x11(title, body, mode, duration)
    }

    pub fn capture_screen_jpeg() -> Result<Vec<u8>, String> {
        capture_via_scrot(75, "1280x")
    }

    pub fn capture_frame_jpeg(quality: u32, thumb: bool) -> Option<Vec<u8>> {
        let resize = if thumb { "320x" } else { "1280x" };
        capture_via_scrot(quality, resize).ok()
    }

    fn capture_via_scrot(quality: u32, resize: &str) -> Result<Vec<u8>, String> {
        let pid = std::process::id();
        let tmp_png = format!("{}/lab-snap-{}.png", super::tmp_dir(), pid);
        let tmp_jpg = format!("{}/lab-snap-{}.jpg", super::tmp_dir(), pid);
        let out = std::process::Command::new("scrot")
            .args(["-o", &tmp_png])
            .output()
            .map_err(|e| format!("scrot 不可用: {}", e))?;
        if !out.status.success() {
            return Err(format!("scrot 失败: {}", String::from_utf8_lossy(&out.stderr).trim()));
        }
        let conv = std::process::Command::new("convert")
            .args([&tmp_png, "-quality", &quality.to_string(), "-resize", resize, &tmp_jpg])
            .output()
            .map_err(|e| format!("convert 不可用: {}", e))?;
        std::fs::remove_file(&tmp_png).ok();
        if !conv.status.success() {
            std::fs::remove_file(&tmp_jpg).ok();
            return Err(format!("JPEG 转换失败: {}", String::from_utf8_lossy(&conv.stderr).trim()));
        }
        let data = std::fs::read(&tmp_jpg).map_err(|e| format!("读取 JPEG 失败: {}", e))?;
        std::fs::remove_file(&tmp_jpg).ok();
        if data.is_empty() {
            return Err("截图文件为空".to_string());
        }
        Ok(data)
    }

    pub async fn send_input(params: &Value) -> String {
        let input_type = params.get("type").and_then(|v| v.as_str()).unwrap_or("");
        match input_type {
            "mouse_move" => {
                let x = params.get("x").and_then(|v| v.as_i64()).unwrap_or(0);
                let y = params.get("y").and_then(|v| v.as_i64()).unwrap_or(0);
                run_xdotool(&["mousemove", &x.to_string(), &y.to_string()]).await
            }
            "mouse_click" => {
                let x = params.get("x").and_then(|v| v.as_i64()).unwrap_or(0);
                let y = params.get("y").and_then(|v| v.as_i64()).unwrap_or(0);
                let button = params.get("button").and_then(|v| v.as_u64()).unwrap_or(1);
                let b = button.to_string();
                run_xdotool(&["mousemove", &x.to_string(), &y.to_string(), "click", &b]).await
            }
            "mouse_down" => {
                let button = params.get("button").and_then(|v| v.as_u64()).unwrap_or(1);
                run_xdotool(&["mousedown", &button.to_string()]).await
            }
            "mouse_up" => {
                let button = params.get("button").and_then(|v| v.as_u64()).unwrap_or(1);
                run_xdotool(&["mouseup", &button.to_string()]).await
            }
            "key" => {
                let key = params.get("key").and_then(|v| v.as_str()).unwrap_or("");
                if key.is_empty() { return "error_no_key".to_string(); }
                run_xdotool(&["key", key]).await
            }
            "type" => {
                let text = params.get("text").and_then(|v| v.as_str()).unwrap_or("");
                if text.is_empty() { return "error_no_text".to_string(); }
                run_xdotool(&["type", "--delay", "20", text]).await
            }
            "scroll" => {
                let dir = params.get("direction").and_then(|v| v.as_str()).unwrap_or("down");
                let button = if dir == "up" { "4" } else { "5" };
                run_xdotool(&["click", button]).await
            }
            _ => format!("unknown_input_type:{}", input_type),
        }
    }

    async fn run_xdotool(args: &[&str]) -> String {
        match Command::new("xdotool").args(args).output().await {
            Ok(o) if o.status.success() => "ok".to_string(),
            Ok(o) => format!("xdotool_error:{}", String::from_utf8_lossy(&o.stderr).trim()),
            Err(e) => format!("xdotool_exec_error:{}", e),
        }
    }

    pub fn get_active_window_sync() -> String {
        let out = std::process::Command::new("xdotool")
            .args(["getactivewindow"])
            .output();
        if let Ok(o) = out {
            if o.status.success() {
                let w = String::from_utf8_lossy(&o.stdout).trim().to_string();
                if !w.is_empty() { return w; }
            }
        }
        // fallback: getwindowfocus（部分 WM 不设置 _NET_ACTIVE_WINDOW）
        std::process::Command::new("xdotool")
            .args(["getwindowfocus"])
            .output()
            .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
            .unwrap_or_default()
    }

    pub fn get_window_name_sync(window_id: &str) -> String {
        if window_id.is_empty() { return String::new(); }
        std::process::Command::new("xdotool")
            .args(["getwindowname", window_id])
            .output()
            .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
            .unwrap_or_default()
    }

    pub fn activate_window_sync(window_id: &str) -> bool {
        std::process::Command::new("xdotool")
            .args(["windowactivate", window_id])
            .output()
            .map(|o| o.status.success())
            .unwrap_or(false)
    }

    pub fn execute_power(reboot: bool) {
        if std::env::var("LAB_POWER_DRYRUN").map(|v| v == "1").unwrap_or(false) {
            println!("[Agent] DRYRUN: 跳过真实{}", if reboot { "重启" } else { "关机" });
            return;
        }
        let cmds: &[&[&str]] = if reboot {
            &[&["systemctl", "reboot"], &["shutdown", "-r", "now"], &["reboot"]]
        } else {
            &[&["systemctl", "poweroff"], &["shutdown", "-h", "now"], &["poweroff"]]
        };
        for c in cmds {
            match std::process::Command::new(c[0]).args(&c[1..]).output() {
                Ok(o) if o.status.success() => {
                    println!("[Agent] 电源指令已执行: {:?}", c);
                    return;
                }
                Ok(o) => println!("[Agent] 电源指令失败 {:?}: {}", c, String::from_utf8_lossy(&o.stderr).trim()),
                Err(e) => println!("[Agent] 电源指令无法启动 {:?}: {}", c, e),
            }
        }
        println!("[Agent] 所有电源指令均失败");
    }

    pub async fn session_logoff(user: &str, _mode: &str) -> Result<String, String> {
        // 定位会话用户：优先指定 user；否则取 loginctl 第一个活跃会话的 USER
        let target_user = if !user.is_empty() {
            user.to_string()
        } else {
            match Command::new("loginctl").args(["list-sessions", "--no-legend"]).output().await {
                Ok(o) if o.status.success() => {
                    let txt = String::from_utf8_lossy(&o.stdout);
                    txt.lines().next()
                        .and_then(|l| l.split_whitespace().nth(2))
                        .unwrap_or("")
                        .trim()
                        .to_string()
                }
                _ => return Err("loginctl 不可用".to_string()),
            }
        };
        if target_user.is_empty() {
            return Err("未找到活跃会话用户".to_string());
        }
        match Command::new("loginctl").args(["terminate-user", &target_user]).output().await {
            Ok(o) if o.status.success() => Ok(format!("已注销会话: {}", target_user)),
            Ok(o) => Err(format!("loginctl 失败: {}", String::from_utf8_lossy(&o.stderr).trim())),
            Err(e) => Err(format!("loginctl 不可用: {}", e)),
        }
    }

    pub async fn run_shell(cmd: &str) -> Result<std::process::Output, String> {
        Command::new("sh").arg("-c").arg(cmd).output().await
            .map_err(|e| format!("启动失败: {}", e))
    }

    pub fn open_target(typ: &str, target: &str) -> Result<String, String> {
        let (prog, args): (String, Vec<String>) = if typ == "url" {
            ("xdg-open".to_string(), vec![target.to_string()])
        } else {
            let mut parts = target.split_whitespace();
            let prog = parts.next().unwrap_or("").to_string();
            let args: Vec<String> = parts.map(|s| s.to_string()).collect();
            (prog, args)
        };
        std::process::Command::new(&prog).args(&args)
            .stdin(std::process::Stdio::null())
            .stdout(std::process::Stdio::null())
            .stderr(std::process::Stdio::null())
            .spawn()
            .map_err(|e| format!("启动失败: {}", e))?;
        Ok(format!("已启动: {} {}", prog, args.join(" ")))
    }

    pub fn replace_running_exe(tmp: &str, exe: &str) -> Result<(), String> {
        std::fs::rename(tmp, exe).map_err(|e| format!("替换二进制失败: {}", e))
    }

    fn which_bin(name: &str) -> Result<String, ()> {
        for dir in std::env::var("PATH").unwrap_or_default().split(':') {
            let p = format!("{}/{}", dir, name);
            if std::path::Path::new(&p).exists() {
                return Ok(p);
            }
        }
        Err(())
    }

    /// X11 覆盖层版消息弹窗（蓝色主题窗口，按任意键确认）
    fn message_overlay_x11(title: &str, body: &str, mode: &str, duration: u64) -> bool {
        use x11_dl::xlib::*;
        let xlib = match Xlib::open() { Ok(x) => x, Err(_) => return false };
        unsafe {
            let dpy = (xlib.XOpenDisplay)(std::ptr::null());
            if dpy.is_null() { return false; }
            let screen = (xlib.XDefaultScreen)(dpy);
            let root = (xlib.XRootWindow)(dpy, screen);
            let dw = (xlib.XDisplayWidth)(dpy, screen) as u32;
            let dh = (xlib.XDisplayHeight)(dpy, screen) as u32;
            let win_w = 520u32.min(dw - 20);
            let win_h = 220u32.min(dh - 20);
            let x = (dw - win_w) as i32 / 2;
            let y = (dh - win_h) as i32 / 4;
            let win = (xlib.XCreateSimpleWindow)(dpy, root, x, y, win_w, win_h, 2,
                0x00336699, 0x00f5f8ff);
            let mut attrs: XSetWindowAttributes = std::mem::zeroed();
            attrs.override_redirect = 1;
            (xlib.XChangeWindowAttributes)(dpy, win, CWOverrideRedirect, &mut attrs);
            (xlib.XSelectInput)(dpy, win, ExposureMask | KeyPressMask | ButtonPressMask);
            (xlib.XMapRaised)(dpy, win);
            (xlib.XRaiseWindow)(dpy, win);

            let gc = (xlib.XDefaultGC)(dpy, screen);
            let font = (xlib.XLoadFont)(dpy, b"fixed\0".as_ptr() as *const std::os::raw::c_char);
            (xlib.XSetFont)(dpy, gc, font);
            let ascii_of = |s: &str| -> String {
                let a: String = s.chars().filter(|c| c.is_ascii()).collect();
                if a.trim().is_empty() { "[Message - see teacher console]".to_string() } else { a }
            };
            let t = ascii_of(title);
            let b = ascii_of(body);
            let hint = if mode == "confirm" { "Press any key to confirm >>" } else { "" };

            let start = std::time::Instant::now();
            let conn_fd = (xlib.XConnectionNumber)(dpy);
            let mut redraw = true;
            loop {
                if mode != "confirm" && start.elapsed().as_secs() >= duration { break; }
                if redraw {
                    (xlib.XSetForeground)(dpy, gc, 0x001a3355);
                    (xlib.XDrawString)(dpy, win, gc, 20, 40, t.as_ptr() as *const _, t.len() as i32);
                    (xlib.XDrawString)(dpy, win, gc, 20, 80, b.as_ptr() as *const _, b.len() as i32);
                    if !hint.is_empty() {
                        (xlib.XDrawString)(dpy, win, gc, 20, win_h as i32 - 24, hint.as_ptr() as *const _, hint.len() as i32);
                    }
                    (xlib.XFlush)(dpy);
                    redraw = false;
                }
                let mut rfds: libc::fd_set = std::mem::zeroed();
                libc::FD_ZERO(&mut rfds);
                libc::FD_SET(conn_fd, &mut rfds);
                let mut tv = libc::timeval { tv_sec: 0, tv_usec: 200_000 };
                libc::select(conn_fd + 1, &mut rfds, std::ptr::null_mut(), std::ptr::null_mut(), &mut tv);
                while (xlib.XPending)(dpy) > 0 {
                    let mut ev: XEvent = std::mem::zeroed();
                    (xlib.XNextEvent)(dpy, &mut ev);
                    if mode == "confirm" && (ev.get_type() == KeyPress || ev.get_type() == ButtonPress) {
                        (xlib.XDestroyWindow)(dpy, win);
                        (xlib.XCloseDisplay)(dpy);
                        return true;
                    }
                    if ev.get_type() == Expose { redraw = true; }
                }
            }
            (xlib.XDestroyWindow)(dpy, win);
            (xlib.XFlush)(dpy);
            (xlib.XCloseDisplay)(dpy);
            mode != "confirm"
        }
    }
}

#[cfg(unix)]
pub use imp::*;

// ================================================================
//                     Windows 实现（Win32 运行时加载）
// ================================================================

#[cfg(target_os = "windows")]
#[allow(dead_code, non_snake_case, non_camel_case_types)]
mod imp {
    use serde_json::Value;
    use std::os::raw::{c_int, c_void};

    // ---------- 基础 Win32 类型（手工 repr，不依赖 winapi crate） ----------
    type BOOL = i32;
    type DWORD = u32;
    type UINT = u32;
    type WCHAR = u16;
    type WPARAM = usize;
    type LPARAM = isize;
    type LRESULT = isize;
    type HANDLE = *mut c_void;
    type HWND = *mut c_void;
    type HDC = *mut c_void;
    type HMODULE = *mut c_void;
    type HBITMAP = *mut c_void;
    type HGDIOBJ = *mut c_void;
    type HBRUSH = *mut c_void;
    type HFONT = *mut c_void;
    type HHOOK = *mut c_void;
    type HOOKPROC = unsafe extern "system" fn(c_int, WPARAM, LPARAM) -> LRESULT;
    type WNDPROC = unsafe extern "system" fn(HWND, UINT, WPARAM, LPARAM) -> LRESULT;

    #[repr(C)]
    #[derive(Clone, Copy)]
    struct POINT { x: i32, y: i32 }

    #[repr(C)]
    #[derive(Clone, Copy)]
    struct RECT { left: i32, top: i32, right: i32, bottom: i32 }

    #[repr(C)]
    struct MSG { hwnd: HWND, message: UINT, wParam: WPARAM, lParam: LPARAM, time: DWORD, pt: POINT }

    #[repr(C)]
    struct WNDCLASSW {
        style: UINT, lpfnWndProc: WNDPROC, cbClsExtra: c_int, cbWndExtra: c_int,
        hInstance: HANDLE, hIcon: HANDLE, hCursor: HANDLE, hbrBackground: HBRUSH,
        lpszMenuName: *const WCHAR, lpszClassName: *const WCHAR,
    }

    #[repr(C)]
    struct GUID { d1: u32, d2: u16, d3: u16, d4: [u8; 8] }

    // ---------- kernel32 静态链接（std 本身依赖 kernel32，必然可用） ----------
    #[link(name = "kernel32")]
    extern "system" {
        fn LoadLibraryW(name: *const WCHAR) -> HMODULE;
        fn GetProcAddress(h: HMODULE, name: *const u8) -> *mut c_void;
        fn GetModuleHandleW(name: *const WCHAR) -> HMODULE;
        fn GetCurrentThreadId() -> DWORD;
        fn GetLastError() -> DWORD;
    }

    /// 运行时取 DLL 导出函数（user32/gdi32/gdiplus/d3d11/advapi32/wtsapi32…）
    unsafe fn sym(dll: &str, func: &str) -> Option<*mut c_void> {
        let w = to_wide(dll);
        let lib = LoadLibraryW(w.as_ptr());
        if lib.is_null() { return None; }
        let mut name = func.as_bytes().to_vec();
        name.push(0);
        let p = GetProcAddress(lib, name.as_ptr());
        if p.is_null() { None } else { Some(p) }
    }

    fn to_wide(s: &str) -> Vec<u16> {
        let mut v: Vec<u16> = s.encode_utf16().collect();
        v.push(0);
        v
    }

    fn from_wide(p: *const u16, max: usize) -> String {
        let mut v = Vec::new();
        unsafe {
            for i in 0..max {
                let c = *p.add(i);
                if c == 0 { break; }
                v.push(c);
            }
        }
        String::from_utf16_lossy(&v)
    }

    // ---------- 图形环境预检 ----------
    pub fn display_available() -> Result<(), String> {
        unsafe {
            // 服务若运行在 Session 0（无交互桌面）则无法弹窗/截屏
            if let Some(p) = sym("kernel32.dll", "WTSGetActiveConsoleSessionId") {
                let f: unsafe extern "system" fn() -> DWORD = std::mem::transmute(p);
                if f() == 0xFFFF_FFFF {
                    return Err("无活动控制台会话（Agent 可能在 Session 0 服务环境）".to_string());
                }
            }
            if sym("user32.dll", "GetDC").is_none() {
                return Err("无法加载 user32.dll".to_string());
            }
        }
        Ok(())
    }

    // ---------- 屏幕采集 ----------

    /// BGRA 缓冲：DXGI Desktop Duplication 主路径
    unsafe fn capture_dxgi_bgra() -> Option<(Vec<u8>, u32, u32)> {
        // ---- COM vtable 槽位（IUnknown: 0 QI / 1 AddRef / 2 Release；
        //      IDXGIObject: 3 SetPrivateData 4 SetPrivateDataInterface 5 GetPrivateData 6 GetParent）----
        // IDXGIDevice: 7 GetAdapter（3 IUnknown + 4 IDXGIObject 之后第一个方法）
        // IDXGIAdapter: 7 EnumOutputs 8 GetDesc
        // IDXGIOutput: 7 GetDesc（继承 IDXGIObject，方法从 slot 7 起）
        // IDXGIOutput1: 18 GetDisplayModeList1 19 FindClosestMatchingMode1
        //               20 GetDisplaySurfaceData1 21 DuplicateOutput
        // IDXGIOutputDuplication: 3 GetDesc 4 AcquireNextFrame ... 10 ReleaseFrame
        // ID3D11Device: 5 CreateTexture2D, 40 GetImmediateContext（直接继承 IUnknown）
        // ID3D11DeviceContext: 14 Map, 15 Unmap, 98 CopyResource
        const IID_IDXGIDEVICE: GUID = GUID { d1: 0x54ec77fa, d2: 0x1377, d3: 0x44e6, d4: [0x8c, 0x32, 0x88, 0xfd, 0x5f, 0x44, 0xc8, 0x4c] };
        const IID_IDXGIFACTORY1: GUID = GUID { d1: 0x770aae78, d2: 0xf26f, d3: 0x4dba, d4: [0xa8, 0x29, 0x25, 0x3c, 0x83, 0xd1, 0xb3, 0x87] };
        const IID_TEX2D: GUID = GUID { d1: 0x6f15aaf2, d2: 0xd208, d3: 0x4e89, d4: [0x9a, 0xb4, 0x48, 0x95, 0x35, 0xd3, 0x4f, 0x9c] };

        type ComObj = *mut c_void;

        unsafe fn qi<T>(obj: ComObj, iid: &GUID, out: *mut *mut T) -> i32 {
            let vtbl = *(obj as *const *const *const c_void);
            let f: unsafe extern "system" fn(ComObj, *const GUID, *mut *mut c_void) -> i32 =
                std::mem::transmute(*vtbl.add(0));
            f(obj, iid, out as *mut *mut c_void)
        }
        unsafe fn release(obj: ComObj) {
            if obj.is_null() { return; }
            let vtbl = *(obj as *const *const *const c_void);
            let f: unsafe extern "system" fn(ComObj) -> u32 = std::mem::transmute(*vtbl.add(2));
            f(obj);
        }
        unsafe fn slot<F: Sized>(obj: ComObj, n: usize) -> F {
            let vtbl = *(obj as *const *const *const c_void);
            let p: *const c_void = *vtbl.add(n);
            std::mem::transmute_copy(&p)
        }

        // 1) D3D11CreateDevice
        let p_create = sym("d3d11.dll", "D3D11CreateDevice")?;
        let d3d_create: unsafe extern "system" fn(
            ComObj, u32, HMODULE, u32, *const u32, u32, u32,
            *mut ComObj, *mut u32, *mut ComObj,
        ) -> i32 = std::mem::transmute(p_create);
        let levels = [0xb000u32, 0xa100, 0xa000, 0x9300, 0x9200, 0x9100];
        let mut dev: ComObj = std::ptr::null_mut();
        let mut ctx: ComObj = std::ptr::null_mut();
        let mut lvl: u32 = 0;
        let hr = d3d_create(std::ptr::null_mut(), 1 /*HARDWARE*/, std::ptr::null_mut(), 0,
            levels.as_ptr(), levels.len() as u32, 7, &mut dev, &mut lvl, &mut ctx);
        if hr < 0 || dev.is_null() { return None; }
        release(ctx);

        // 2) dev->QueryInterface(IDXGIDevice)
        let mut dxgi_dev: ComObj = std::ptr::null_mut();
        if qi(dev, &IID_IDXGIDEVICE, &mut dxgi_dev) < 0 || dxgi_dev.is_null() {
            release(dev); return None;
        }
        // 3) IDXGIDevice::GetAdapter (slot 7)
        let f_adapter: unsafe extern "system" fn(ComObj, *mut ComObj) -> i32 = slot(dxgi_dev, 7);
        let mut adapter: ComObj = std::ptr::null_mut();
        if f_adapter(dxgi_dev, &mut adapter) < 0 || adapter.is_null() {
            release(dxgi_dev); release(dev); return None;
        }
        // 4) adapter->GetParent(IDXGIFactory1) (IDXGIObject slot 6)
        let f_parent: unsafe extern "system" fn(ComObj, *const GUID, *mut ComObj) -> i32 = slot(adapter, 6);
        let mut factory: ComObj = std::ptr::null_mut();
        if f_parent(adapter, &IID_IDXGIFACTORY1, &mut factory) < 0 || factory.is_null() {
            release(adapter); release(dxgi_dev); release(dev); return None;
        }
        // 5) factory->EnumAdapters(0) (slot 3) —— 与上面 GetAdapter 等价，直接复用 adapter
        release(factory);
        // 6) adapter->EnumOutputs(0) (slot 7)
        let f_enum_out: unsafe extern "system" fn(ComObj, u32, *mut ComObj) -> i32 = slot(adapter, 7);
        let mut output: ComObj = std::ptr::null_mut();
        if f_enum_out(adapter, 0, &mut output) < 0 || output.is_null() {
            release(adapter); release(dxgi_dev); release(dev); return None;
        }
        // 7) output->GetDesc (IDXGIOutput slot 7) 取桌面区域
        #[repr(C)]
        struct DXGI_OUTPUT_DESC {
            device_name: [u16; 32], desktop_coordinates: RECT,
            attached_to_desktop: BOOL, rotation: u32, monitor: HANDLE,
        }
        let mut desc: std::mem::MaybeUninit<DXGI_OUTPUT_DESC> = std::mem::MaybeUninit::uninit();
        let f_desc: unsafe extern "system" fn(ComObj, *mut DXGI_OUTPUT_DESC) -> i32 = slot(output, 7);
        let _ = f_desc(output, desc.as_mut_ptr());
        let desc = desc.assume_init();
        let w = (desc.desktop_coordinates.right - desc.desktop_coordinates.left) as u32;
        let h = (desc.desktop_coordinates.bottom - desc.desktop_coordinates.top) as u32;
        if w == 0 || h == 0 {
            release(output); release(adapter); release(dxgi_dev); release(dev); return None;
        }
        // 8) IDXGIOutput1::DuplicateOutput (slot 21)
        let f_dup: unsafe extern "system" fn(ComObj, ComObj, *mut ComObj) -> i32 = slot(output, 21);
        let mut dup: ComObj = std::ptr::null_mut();
        if f_dup(output, dev, &mut dup) < 0 || dup.is_null() {
            release(output); release(adapter); release(dxgi_dev); release(dev); return None;
        }
        // 9) AcquireNextFrame (slot 4)：(TimeoutInMs, *pFrameInfo, **ppDesktopResource)
        let mut frame_info = [0u8; 32]; // DXGI_OUTDUPL_FRAME_INFO = 32 bytes
        let mut res: ComObj = std::ptr::null_mut();
        let f_acq: unsafe extern "system" fn(ComObj, u32, *mut [u8; 32], *mut ComObj) -> i32 = slot(dup, 4);
        let hr = f_acq(dup, 500, &mut frame_info, &mut res);
        if hr < 0 || res.is_null() {
            release(dup); release(output); release(adapter); release(dxgi_dev); release(dev); return None;
        }
        // 10) res(IDXGIResource)->QI(ID3D11Texture2D)
        let mut tex: ComObj = std::ptr::null_mut();
        if qi(res, &IID_TEX2D, &mut tex) < 0 || tex.is_null() {
            release(res); release(dup); release(output); release(adapter); release(dxgi_dev); release(dev);
            return None;
        }
        // 11) staging 纹理 + CopyResource + Map 读取
        #[repr(C)]
        struct TEX2D_DESC {
            width: u32, height: u32, mip: u32, arr: u32, fmt: u32,
            sample_count: u32, sample_quality: u32, usage: u32, bind: u32, cpu: u32, misc: u32,
        }
        let stg_desc = TEX2D_DESC {
            width: w, height: h, mip: 1, arr: 1,
            fmt: 87 /*B8G8R8A8_UNORM*/, sample_count: 1, sample_quality: 0,
            usage: 3 /*STAGING*/, bind: 0, cpu: 1 /*READ*/, misc: 0,
        };
        let mut staging: ComObj = std::ptr::null_mut();
        let f_ct: unsafe extern "system" fn(ComObj, *const TEX2D_DESC, *const c_void, *mut ComObj) -> i32 = slot(dev, 5);
        if f_ct(dev, &stg_desc, std::ptr::null(), &mut staging) < 0 || staging.is_null() {
            release(tex); release(res); release(dup); release(output); release(adapter); release(dxgi_dev); release(dev);
            return None;
        }
        let mut ctx2: ComObj = std::ptr::null_mut();
        let f_gic: unsafe extern "system" fn(ComObj, *mut ComObj) -> i32 = slot(dev, 40);
        if f_gic(dev, &mut ctx2) < 0 || ctx2.is_null() {
            release(staging); release(tex); release(res); release(dup); release(output); release(adapter); release(dxgi_dev); release(dev);
            return None;
        }
        let f_copy: unsafe extern "system" fn(ComObj, ComObj, ComObj) = slot(ctx2, 98);
        f_copy(ctx2, staging, tex);

        #[repr(C)]
        struct MAPPED { data: *mut u8, row_pitch: u32, depth_pitch: u32 }
        let mut mapped: MAPPED = MAPPED { data: std::ptr::null_mut(), row_pitch: 0, depth_pitch: 0 };
        let f_map: unsafe extern "system" fn(ComObj, ComObj, u32, u32, u32, *mut MAPPED) -> i32 = slot(ctx2, 14);
        let hr = f_map(ctx2, staging, 0, 1 /*READ*/, 0, &mut mapped);
        let result = if hr >= 0 && !mapped.data.is_null() {
            let mut buf = vec![0u8; (w * h * 4) as usize];
            for row in 0..h {
                let src = mapped.data.add((row * mapped.row_pitch) as usize);
                let dst = (row * w * 4) as usize;
                std::ptr::copy_nonoverlapping(src, buf.as_mut_ptr().add(dst), (w * 4) as usize);
            }
            let f_unmap: unsafe extern "system" fn(ComObj, ComObj, u32) = slot(ctx2, 15);
            f_unmap(ctx2, staging, 0);
            Some((buf, w, h))
        } else {
            None
        };
        // 12) ReleaseFrame (slot 10) + 全部释放
        let f_relf: unsafe extern "system" fn(ComObj) -> i32 = slot(dup, 10);
        f_relf(dup);
        release(ctx2); release(staging); release(tex); release(res); release(dup);
        release(output); release(adapter); release(dxgi_dev); release(dev);
        result
    }

    /// GDI BitBlt 备路径：BGRA
    unsafe fn capture_gdi_bgra() -> Option<(Vec<u8>, u32, u32)> {
        let p_gdc = sym("user32.dll", "GetDC")?;
        let p_rdc = sym("user32.dll", "ReleaseDC")?;
        let p_cdc = sym("gdi32.dll", "CreateCompatibleDC")?;
        let p_ddc = sym("gdi32.dll", "DeleteDC")?;
        let p_cbb = sym("gdi32.dll", "CreateCompatibleBitmap")?;
        let p_sel = sym("gdi32.dll", "SelectObject")?;
        let p_del = sym("gdi32.dll", "DeleteObject")?;
        let p_blt = sym("gdi32.dll", "BitBlt")?;
        let p_gdi = sym("gdi32.dll", "GetDIBits")?;
        let p_sm = sym("user32.dll", "GetSystemMetrics")?;

        let get_sm: unsafe extern "system" fn(c_int) -> c_int = std::mem::transmute(p_sm);
        let w = get_sm(0) as u32; // SM_CXSCREEN
        let h = get_sm(1) as u32; // SM_CYSCREEN
        if w == 0 || h == 0 { return None; }

        let get_dc: unsafe extern "system" fn(HWND) -> HDC = std::mem::transmute(p_gdc);
        let rel_dc: unsafe extern "system" fn(HWND, HDC) -> c_int = std::mem::transmute(p_rdc);
        let create_dc: unsafe extern "system" fn(HDC) -> HDC = std::mem::transmute(p_cdc);
        let del_dc: unsafe extern "system" fn(HDC) -> BOOL = std::mem::transmute(p_ddc);
        let create_bmp: unsafe extern "system" fn(HDC, c_int, c_int) -> HBITMAP = std::mem::transmute(p_cbb);
        let select_obj: unsafe extern "system" fn(HDC, HGDIOBJ) -> HGDIOBJ = std::mem::transmute(p_sel);
        let del_obj: unsafe extern "system" fn(HGDIOBJ) -> BOOL = std::mem::transmute(p_del);
        let bit_blt: unsafe extern "system" fn(HDC, c_int, c_int, c_int, c_int, HDC, c_int, c_int, DWORD) -> BOOL = std::mem::transmute(p_blt);

        #[repr(C)]
        struct BITMAPINFOHEADER {
            biSize: u32, biWidth: i32, biHeight: i32, biPlanes: u16, biBitCount: u16,
            biCompression: u32, biSizeImage: u32, biXPelsPerMeter: i32, biYPelsPerMeter: i32,
            biClrUsed: u32, biClrImportant: u32,
        }
        #[repr(C)]
        struct BITMAPINFO { hdr: BITMAPINFOHEADER, colors: [u32; 1] }
        let get_dib: unsafe extern "system" fn(HDC, HBITMAP, c_int, c_int, *mut c_void, *mut BITMAPINFO, UINT, *mut c_void) -> c_int = std::mem::transmute(p_gdi);

        let screen = get_dc(std::ptr::null_mut());
        if screen.is_null() { return None; }
        let mem = create_dc(screen);
        let bmp = create_bmp(screen, w as c_int, h as c_int);
        let mut out: Option<(Vec<u8>, u32, u32)> = None;
        if !mem.is_null() && !bmp.is_null() {
            let old = select_obj(mem, bmp as HGDIOBJ);
            if bit_blt(mem, 0, 0, w as c_int, h as c_int, screen, 0, 0, 0x00CC_0020 /*SRCCOPY*/) != 0 {
                let mut bi = BITMAPINFO {
                    hdr: BITMAPINFOHEADER {
                        biSize: 40, biWidth: w as i32, biHeight: -(h as i32), // top-down
                        biPlanes: 1, biBitCount: 32, biCompression: 0,
                        biSizeImage: 0, biXPelsPerMeter: 0, biYPelsPerMeter: 0,
                        biClrUsed: 0, biClrImportant: 0,
                    },
                    colors: [0],
                };
                let mut buf = vec![0u8; (w * h * 4) as usize];
                let lines = get_dib(screen, bmp, 0, h as c_int, buf.as_mut_ptr() as *mut c_void,
                    &mut bi, 0 /*DIB_RGB_COLORS*/, std::ptr::null_mut());
                if lines > 0 {
                    out = Some((buf, w, h));
                }
            }
            select_obj(mem, old);
        }
        if !bmp.is_null() { del_obj(bmp as HGDIOBJ); }
        if !mem.is_null() { del_dc(mem); }
        rel_dc(std::ptr::null_mut(), screen);
        out
    }

    /// BGRA → JPEG（GDI+ 运行时加载）；失败降级 BMP
    unsafe fn encode_image(bgra: &[u8], w: u32, h: u32, quality: u32) -> Vec<u8> {
        encode_jpeg_gdiplus(bgra, w, h, quality).unwrap_or_else(|| encode_bmp(bgra, w, h))
    }

    unsafe fn encode_jpeg_gdiplus(bgra: &[u8], w: u32, h: u32, quality: u32) -> Option<Vec<u8>> {
        let p_start = sym("gdiplus.dll", "GdiplusStartup")?;
        let p_shutdown = sym("gdiplus.dll", "GdiplusShutdown")?;
        let p_ges = sym("gdiplus.dll", "GdipGetImageEncodersSize")?;
        let p_ge = sym("gdiplus.dll", "GdipGetImageEncoders")?;
        let p_cbfs = sym("gdiplus.dll", "GdipCreateBitmapFromScan0")?;
        let p_save = sym("gdiplus.dll", "GdipSaveImageToFile")?;
        let p_di = sym("gdiplus.dll", "GdipDisposeImage")?;

        #[repr(C)]
        struct GP_START_IN { version: u32, _p1: *mut c_void, _p2: *mut c_void, _p3: *mut c_void }

        let startup: unsafe extern "system" fn(*mut usize, *mut GP_START_IN, *mut c_void) -> c_int =
            std::mem::transmute(p_start);
        let shutdown: unsafe extern "system" fn(usize) = std::mem::transmute(p_shutdown);
        let mut token: usize = 0;
        let mut input = GP_START_IN { version: 1, _p1: std::ptr::null_mut(), _p2: std::ptr::null_mut(), _p3: std::ptr::null_mut() };
        if startup(&mut token, &mut input, std::ptr::null_mut()) != 0 {
            return None;
        }

        let create_bmp: unsafe extern "system" fn(c_int, c_int, c_int, c_int, *const u8, *mut *mut c_void) -> c_int =
            std::mem::transmute(p_cbfs);
        let mut img: *mut c_void = std::ptr::null_mut();
        let stride = (w * 4) as c_int;
        if create_bmp(w as c_int, h as c_int, stride, 2498570 /*PixelFormat32bppARGB*/, bgra.as_ptr(), &mut img) != 0 || img.is_null() {
            shutdown(token);
            return None;
        }

        // 找 JPEG 编码器（ImageCodecInfo.MimeType 指针偏移 64）
        let get_size: unsafe extern "system" fn(*mut UINT, *mut UINT) -> c_int = std::mem::transmute(p_ges);
        let get_enc: unsafe extern "system" fn(UINT, UINT, *mut c_void, *mut UINT) -> c_int = std::mem::transmute(p_ge);
        let mut n: UINT = 0;
        let mut sz: UINT = 0;
        let mut jpeg_cls: Option<GUID> = None;
        if get_size(&mut n, &mut sz) == 0 && n > 0 {
            let mut buf = vec![0u8; sz as usize];
            if get_enc(n, sz, buf.as_mut_ptr() as *mut c_void, &mut sz) == 0 {
                let entry_size = (sz / n) as usize;
                for i in 0..n as usize {
                    let base = buf.as_ptr().add(i * entry_size);
                    let mime_ptr = *((base.add(64)) as *const *const WCHAR);
                    if from_wide(mime_ptr, 64) == "image/jpeg" {
                        let mut g: GUID = std::mem::zeroed();
                        std::ptr::copy_nonoverlapping(base, &mut g as *mut GUID as *mut u8, std::mem::size_of::<GUID>());
                        jpeg_cls = Some(g);
                        break;
                    }
                }
            }
        }

        let mut bytes: Option<Vec<u8>> = None;
        if let Some(cls) = jpeg_cls {
            #[repr(C)]
            struct ENC_PARAM { guid: GUID, count: u32, vtype: u32, value: *mut c_void }
            #[repr(C)]
            struct ENC_PARAMS { count: UINT, params: ENC_PARAM }
            let mut q: u32 = quality;
            let param = ENC_PARAM {
                guid: GUID { d1: 0x1d5be4b5, d2: 0xfa4e, d3: 0x452d, d4: [0x9c, 0x9c, 0x5b, 0x18, 0x52, 0x38, 0x4a, 0xd5] },
                count: 1, vtype: 4 /*ValueDataTypeLong*/, value: &mut q as *mut u32 as *mut c_void,
            };
            let mut params = ENC_PARAMS { count: 1, params: param };
            let tmp = format!("{}\\lab-snap-{}.jpg", super::tmp_dir(), std::process::id());
            let wtmp = to_wide(&tmp);
            let save: unsafe extern "system" fn(*mut c_void, *const WCHAR, *const GUID, *mut ENC_PARAMS) -> c_int =
                std::mem::transmute(p_save);
            if save(img, wtmp.as_ptr(), &cls, &mut params) == 0 {
                bytes = std::fs::read(&tmp).ok();
                std::fs::remove_file(&tmp).ok();
            }
        }
        let dispose: unsafe extern "system" fn(*mut c_void) -> c_int = std::mem::transmute(p_di);
        dispose(img);
        shutdown(token);
        bytes
    }

    /// BMP 兜底编码（无 GDI+ 时）
    fn encode_bmp(bgra: &[u8], w: u32, h: u32) -> Vec<u8> {
        let img_size = (w * h * 4) as usize;
        let mut out = Vec::with_capacity(54 + img_size);
        out.extend_from_slice(b"BM");
        out.extend_from_slice(&((54 + img_size) as u32).to_le_bytes());
        out.extend_from_slice(&[0u8; 4]);
        out.extend_from_slice(&54u32.to_le_bytes());
        out.extend_from_slice(&40u32.to_le_bytes());
        out.extend_from_slice(&(w as i32).to_le_bytes());
        out.extend_from_slice(&(h as i32).to_le_bytes());
        out.extend_from_slice(&1u16.to_le_bytes());
        out.extend_from_slice(&32u16.to_le_bytes());
        out.extend_from_slice(&0u32.to_le_bytes());
        out.extend_from_slice(&(img_size as u32).to_le_bytes());
        out.extend_from_slice(&2835u32.to_le_bytes());
        out.extend_from_slice(&2835u32.to_le_bytes());
        out.extend_from_slice(&0u32.to_le_bytes());
        out.extend_from_slice(&0u32.to_le_bytes());
        out.extend_from_slice(bgra);
        out
    }

    /// 最近邻缩放（BGRA）
    fn resize_bgra(src: &[u8], sw: u32, sh: u32, dw: u32) -> (Vec<u8>, u32, u32) {
        if dw >= sw || sw == 0 || sh == 0 {
            return (src.to_vec(), sw, sh);
        }
        let dh = (sh as u64 * dw as u64 / sw as u64).max(1) as u32;
        let mut out = vec![0u8; (dw * dh * 4) as usize];
        for y in 0..dh {
            let sy = (y as u64 * sh as u64 / dh as u64) as usize;
            for x in 0..dw {
                let sx = (x as u64 * sw as u64 / dw as u64) as usize;
                let si = (sy * sw as usize + sx) * 4;
                let di = (y as usize * dw as usize + x as usize) * 4;
                out[di..di + 4].copy_from_slice(&src[si..si + 4]);
            }
        }
        (out, dw, dh)
    }

    /// DXGI/GDI 采集返回的 alpha 恒为 0，编码前必须置 0xFF，否则 JPEG 全黑
    fn fix_alpha(buf: &mut [u8]) {
        for px in buf.chunks_exact_mut(4) {
            px[3] = 0xFF;
        }
    }

    fn capture_bgra() -> Option<(Vec<u8>, u32, u32)> {
        unsafe {
            let (mut bgra, w, h) = capture_dxgi_bgra()
                .or_else(|| capture_gdi_bgra())?;
            fix_alpha(&mut bgra);
            Some((bgra, w, h))
        }
    }

    pub fn capture_screen_jpeg() -> Result<Vec<u8>, String> {
        let (bgra, w, h) = capture_bgra().ok_or("屏幕采集失败（DXGI/GDI 均不可用）")?;
        let (data, rw, rh) = resize_bgra(&bgra, w, h, 1280);
        Ok(unsafe { encode_image(&data, rw, rh, 75) })
    }

    pub fn capture_frame_jpeg(quality: u32, thumb: bool) -> Option<Vec<u8>> {
        let (bgra, w, h) = capture_bgra()?;
        let target_w = if thumb { 320 } else { 1280 };
        let (data, rw, rh) = resize_bgra(&bgra, w, h, target_w);
        Some(unsafe { encode_image(&data, rw, rh, quality) })
    }

    // ---------- 锁屏覆盖层 ----------
    static LOCK_MSG: std::sync::OnceLock<String> = std::sync::OnceLock::new();

    pub fn lock_overlay_main(cancel_rx: std::sync::mpsc::Receiver<()>, message: String) {
        unsafe {
            let (p_reg, p_cw, p_show, p_gm, p_tm, p_dm, p_st, p_swh, p_uwh, p_sm, p_inv, p_ptm) = match (
                sym("user32.dll", "RegisterClassW"), sym("user32.dll", "CreateWindowExW"),
                sym("user32.dll", "ShowWindow"), sym("user32.dll", "GetMessageW"),
                sym("user32.dll", "TranslateMessage"), sym("user32.dll", "DispatchMessageW"),
                sym("user32.dll", "SetTimer"), sym("user32.dll", "SetWindowsHookExW"),
                sym("user32.dll", "UnhookWindowsHookEx"), sym("user32.dll", "GetSystemMetrics"),
                sym("user32.dll", "InvalidateRect"), sym("user32.dll", "PostThreadMessageW"),
            ) {
                (Some(a), Some(b), Some(c), Some(d), Some(e), Some(f), Some(g), Some(hh), Some(i), Some(j), Some(k), Some(l)) => (a, b, c, d, e, f, g, hh, i, j, k, l),
                _ => { println!("[Agent] 锁屏失败: user32 函数缺失"); return; }
            };

            let register: unsafe extern "system" fn(*const WNDCLASSW) -> u16 = std::mem::transmute(p_reg);
            let create: unsafe extern "system" fn(DWORD, *const WCHAR, *const WCHAR, DWORD, c_int, c_int, c_int, c_int, HWND, HANDLE, HANDLE, *mut c_void) -> HWND = std::mem::transmute(p_cw);
            let show: unsafe extern "system" fn(HWND, c_int) -> BOOL = std::mem::transmute(p_show);
            let get_msg: unsafe extern "system" fn(*mut MSG, HWND, UINT, UINT) -> BOOL = std::mem::transmute(p_gm);
            let trans_msg: unsafe extern "system" fn(*const MSG) -> BOOL = std::mem::transmute(p_tm);
            let dispatch: unsafe extern "system" fn(*const MSG) -> LRESULT = std::mem::transmute(p_dm);
            let set_timer: unsafe extern "system" fn(HWND, usize, UINT, *mut c_void) -> usize = std::mem::transmute(p_st);
            let set_hook: unsafe extern "system" fn(c_int, HOOKPROC, HMODULE, DWORD) -> HHOOK = std::mem::transmute(p_swh);
            let unhook: unsafe extern "system" fn(HHOOK) -> BOOL = std::mem::transmute(p_uwh);
            let get_sm: unsafe extern "system" fn(c_int) -> c_int = std::mem::transmute(p_sm);
            let invalidate: unsafe extern "system" fn(HWND, *const RECT, BOOL) -> BOOL = std::mem::transmute(p_inv);
            let post_thread_msg: unsafe extern "system" fn(DWORD, UINT, WPARAM, LPARAM) -> BOOL = std::mem::transmute(p_ptm);

            LOCK_MSG.get_or_init(|| message.clone());

            let cls_name = to_wide("LabLockOverlay");
            let hinst = GetModuleHandleW(std::ptr::null());
            let wc = WNDCLASSW {
                style: 0x0001 | 0x0002 /*VREDRAW|HREDRAW*/,
                lpfnWndProc: lock_wnd_proc,
                cbClsExtra: 0, cbWndExtra: 0,
                hInstance: hinst, hIcon: std::ptr::null_mut(), hCursor: std::ptr::null_mut(),
                hbrBackground: std::ptr::null_mut(),
                lpszMenuName: std::ptr::null(),
                lpszClassName: cls_name.as_ptr(),
            };
            register(&wc); // 重复注册失败无碍

            let x = get_sm(76); // SM_XVIRTUALSCREEN
            let y = get_sm(77); // SM_YVIRTUALSCREEN
            let w = get_sm(78); // SM_CXVIRTUALSCREEN（覆盖全部显示器）
            let h = get_sm(79); // SM_CYVIRTUALSCREEN
            let title = to_wide("LabTeaching Lock");
            let hwnd = create(
                0x0000_0008 | 0x0000_0080 /*TOPMOST|TOOLWINDOW*/,
                cls_name.as_ptr(), title.as_ptr(),
                0x8000_0000 /*WS_POPUP*/,
                x, y, w, h,
                std::ptr::null_mut(), std::ptr::null_mut(), hinst, std::ptr::null_mut(),
            );
            if hwnd.is_null() {
                println!("[Agent] 锁屏失败: 无法创建覆盖窗口");
                return;
            }
            show(hwnd, 5 /*SW_SHOW*/);
            set_timer(hwnd, 1, 1000, std::ptr::null_mut());

            // 键鼠抢占：低级钩子吞掉全部输入（LL 钩子需要本线程有消息循环）
            let kb_hook = set_hook(13 /*WH_KEYBOARD_LL*/, lock_hook, hinst, 0);
            let ms_hook = set_hook(14 /*WH_MOUSE_LL*/, lock_hook, hinst, 0);

            let max_secs: u64 = std::env::var("LAB_LOCK_MAX_MIN").ok()
                .and_then(|s| s.parse().ok()).unwrap_or(240u64) * 60;
            let start = std::time::Instant::now();

            // 看门狗线程：取消信号/超时 → PostThreadMessage(WM_QUIT)
            let tid = GetCurrentThreadId();
            std::thread::spawn(move || {
                loop {
                    let cancelled = cancel_rx.try_recv().is_ok();
                    let timeout = start.elapsed().as_secs() > max_secs;
                    if cancelled || timeout {
                        if timeout {
                            println!("[Agent] 锁屏超过 {} 分钟，看门狗自动解除", max_secs / 60);
                        }
                        unsafe { post_thread_msg(tid, 0x0012 /*WM_QUIT*/, 0, 0); }
                        break;
                    }
                    std::thread::sleep(std::time::Duration::from_millis(200));
                }
            });

            // 消息循环（兼作 LL 钩子分发）
            let mut msg: MSG = std::mem::zeroed();
            loop {
                let r = get_msg(&mut msg, std::ptr::null_mut(), 0, 0);
                if r == 0 || r == -1 { break; }
                if msg.message == 0x0113 /*WM_TIMER*/ {
                    // 每秒：抢占前台 + 置顶 + 重绘（防绕过）
                    if let (Some(p_fw), Some(p_swp)) = (sym("user32.dll", "SetForegroundWindow"), sym("user32.dll", "SetWindowPos")) {
                        let fw: unsafe extern "system" fn(HWND) -> BOOL = std::mem::transmute(p_fw);
                        let swp: unsafe extern "system" fn(HWND, HWND, c_int, c_int, c_int, c_int, UINT) -> BOOL = std::mem::transmute(p_swp);
                        fw(hwnd);
                        swp(hwnd, (-1isize) as HWND /*HWND_TOPMOST*/, 0, 0, 0, 0, 0x0002 | 0x0001 /*NOMOVE|NOSIZE*/);
                    }
                    invalidate(hwnd, std::ptr::null(), 1);
                    continue;
                }
                trans_msg(&msg);
                dispatch(&msg);
            }
            if !kb_hook.is_null() { unhook(kb_hook); }
            if !ms_hook.is_null() { unhook(ms_hook); }
            println!("[Agent] 锁屏覆盖层已退出");
        }
    }

    unsafe extern "system" fn lock_hook(_code: c_int, _wp: WPARAM, _lp: LPARAM) -> LRESULT {
        1 // 吞掉全部键鼠输入
    }

    unsafe extern "system" fn lock_wnd_proc(hwnd: HWND, msg: UINT, wp: WPARAM, lp: LPARAM) -> LRESULT {
        match msg {
            0x000F /*WM_PAINT*/ => {
                paint_lock(hwnd);
                0
            }
            0x0014 /*WM_ERASEBKGND*/ => 1,
            0x0010 /*WM_CLOSE*/ => 0, // 禁止关闭
            _ => default_proc(hwnd, msg, wp, lp),
        }
    }

    unsafe fn default_proc(hwnd: HWND, msg: UINT, wp: WPARAM, lp: LPARAM) -> LRESULT {
        if let Some(p) = sym("user32.dll", "DefWindowProcW") {
            let f: unsafe extern "system" fn(HWND, UINT, WPARAM, LPARAM) -> LRESULT = std::mem::transmute(p);
            f(hwnd, msg, wp, lp)
        } else {
            0
        }
    }

    /// 锁屏绘制：黑底 + 居中白字（DrawTextW 支持中文）
    unsafe fn paint_lock(hwnd: HWND) {
        let (p_bp, p_ep, p_cr, p_fill, p_cf, p_stc, p_sbm, p_dt, p_sel, p_del, p_cb) = match (
            sym("user32.dll", "BeginPaint"), sym("user32.dll", "EndPaint"),
            sym("user32.dll", "GetClientRect"), sym("user32.dll", "FillRect"),
            sym("gdi32.dll", "CreateFontW"), sym("gdi32.dll", "SetTextColor"),
            sym("gdi32.dll", "SetBkMode"), sym("user32.dll", "DrawTextW"),
            sym("gdi32.dll", "SelectObject"), sym("gdi32.dll", "DeleteObject"),
            sym("gdi32.dll", "CreateSolidBrush"),
        ) {
            (Some(a), Some(b), Some(c), Some(d), Some(e), Some(f), Some(g), Some(hh), Some(i), Some(j), Some(k)) => (a, b, c, d, e, f, g, hh, i, j, k),
            _ => return,
        };

        #[repr(C)]
        struct PAINTSTRUCT { hdc: HDC, erase: BOOL, rc: RECT, f_restore: BOOL, f_inc_update: BOOL, _reserved: [u8; 32] }

        let begin: unsafe extern "system" fn(HWND, *mut PAINTSTRUCT) -> HDC = std::mem::transmute(p_bp);
        let end: unsafe extern "system" fn(HWND, *mut PAINTSTRUCT) -> BOOL = std::mem::transmute(p_ep);
        let client: unsafe extern "system" fn(HWND, *mut RECT) -> BOOL = std::mem::transmute(p_cr);
        let fill: unsafe extern "system" fn(HDC, *const RECT, HBRUSH) -> c_int = std::mem::transmute(p_fill);
        let create_font: unsafe extern "system" fn(c_int, c_int, c_int, c_int, c_int, DWORD, DWORD, DWORD, DWORD, DWORD, DWORD, DWORD, DWORD, *const WCHAR) -> HFONT = std::mem::transmute(p_cf);
        let set_tc: unsafe extern "system" fn(HDC, u32) -> u32 = std::mem::transmute(p_stc);
        let set_bm: unsafe extern "system" fn(HDC, c_int) -> c_int = std::mem::transmute(p_sbm);
        let draw: unsafe extern "system" fn(HDC, *const WCHAR, c_int, *mut RECT, UINT) -> c_int = std::mem::transmute(p_dt);
        let select_obj: unsafe extern "system" fn(HDC, HGDIOBJ) -> HGDIOBJ = std::mem::transmute(p_sel);
        let del_obj: unsafe extern "system" fn(HGDIOBJ) -> BOOL = std::mem::transmute(p_del);
        let brush: unsafe extern "system" fn(u32) -> HBRUSH = std::mem::transmute(p_cb);

        let mut ps: PAINTSTRUCT = std::mem::zeroed();
        let hdc = begin(hwnd, &mut ps);
        if hdc.is_null() { return; }
        let mut rc: RECT = std::mem::zeroed();
        client(hwnd, &mut rc);
        let bg = brush(0x0000_0000);
        fill(hdc, &rc, bg);
        del_obj(bg as HGDIOBJ);

        let font_name = to_wide("Microsoft YaHei");
        let font = create_font(-48, 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 5, 0, font_name.as_ptr());
        let old = select_obj(hdc, font as HGDIOBJ);
        set_tc(hdc, 0xFF_FF_FF);
        set_bm(hdc, 1 /*TRANSPARENT*/);
        let text = format!("屏幕已锁定\n{}", LOCK_MSG.get().map(|s| s.as_str()).unwrap_or(""));
        let wtext = to_wide(&text);
        let mut trc = rc;
        draw(hdc, wtext.as_ptr(), -1, &mut trc, 0x0001 | 0x0004 | 0x0010 /*CENTER|VCENTER|WORDBREAK*/);
        select_obj(hdc, old);
        del_obj(font as HGDIOBJ);
        end(hwnd, &mut ps);
    }

    // ---------- 消息弹窗 ----------
    pub fn message_overlay_sync(title: &str, body: &str, mode: &str, duration: u64) -> bool {
        println!("[Agent] 显示消息弹窗: [{}] {} (mode={}, {}s)", title, body, mode, duration);
        unsafe {
            let t = to_wide(title);
            let b = to_wide(body);
            if mode == "confirm" {
                if let Some(p) = sym("user32.dll", "MessageBoxW") {
                    let mb: unsafe extern "system" fn(HWND, *const WCHAR, *const WCHAR, UINT) -> c_int =
                        std::mem::transmute(p);
                    // MB_OK | MB_ICONINFORMATION | MB_SYSTEMMODAL | MB_SETFOREGROUND
                    let r = mb(std::ptr::null_mut(), b.as_ptr(), t.as_ptr(), 0x0 | 0x40 | 0x1000 | 0x1_0000);
                    return r != 0;
                }
                return false;
            }
            // notify：MessageBoxTimeoutW 定时自动关闭
            if let Some(p) = sym("user32.dll", "MessageBoxTimeoutW") {
                let mbt: unsafe extern "system" fn(HWND, *const WCHAR, *const WCHAR, UINT, u16, DWORD) -> c_int =
                    std::mem::transmute(p);
                let _ = mbt(std::ptr::null_mut(), b.as_ptr(), t.as_ptr(), 0x0 | 0x40 | 0x1000, 0, (duration * 1000) as DWORD);
                return true;
            }
            if let Some(p) = sym("user32.dll", "MessageBoxW") {
                let mb: unsafe extern "system" fn(HWND, *const WCHAR, *const WCHAR, UINT) -> c_int =
                    std::mem::transmute(p);
                let _ = mb(std::ptr::null_mut(), b.as_ptr(), t.as_ptr(), 0x0 | 0x40 | 0x1000);
                return true;
            }
        }
        false
    }

    // ---------- SendInput 键鼠注入 ----------
    #[repr(C)]
    #[derive(Clone, Copy)]
    struct MOUSEINPUT { dx: i32, dy: i32, data: DWORD, flags: DWORD, time: DWORD, extra: usize }
    #[repr(C)]
    #[derive(Clone, Copy)]
    struct KEYBDINPUT { vk: u16, scan: u16, flags: DWORD, time: DWORD, extra: usize }
    #[repr(C)]
    #[derive(Clone, Copy)]
    union INPUT_U { mi: MOUSEINPUT, ki: KEYBDINPUT }
    #[repr(C)]
    #[derive(Clone, Copy)]
    struct INPUT { itype: DWORD, u: INPUT_U }

    fn empty_input() -> INPUT {
        INPUT { itype: 0, u: INPUT_U { mi: MOUSEINPUT { dx: 0, dy: 0, data: 0, flags: 0, time: 0, extra: 0 } } }
    }

    /// xdotool 键名 → Windows 虚拟键码（返回 (vk, 是否扩展键)）
    fn vk_from_name(key: &str) -> Option<(u16, bool)> {
        let k = key.to_lowercase();
        match k.as_str() {
            "return" | "enter" | "kp_enter" => return Some((0x0D, false)),
            "escape" | "esc" => return Some((0x1B, false)),
            "backspace" => return Some((0x08, false)),
            "tab" => return Some((0x09, false)),
            "space" => return Some((0x20, false)),
            "delete" | "del" => return Some((0x2E, true)),
            "insert" => return Some((0x2D, true)),
            "home" => return Some((0x24, true)),
            "end" => return Some((0x23, true)),
            "prior" | "page_up" => return Some((0x21, true)),
            "next" | "page_down" => return Some((0x22, true)),
            "left" => return Some((0x25, true)),
            "up" => return Some((0x26, true)),
            "right" => return Some((0x27, true)),
            "down" => return Some((0x28, true)),
            "f1" => return Some((0x70, false)), "f2" => return Some((0x71, false)),
            "f3" => return Some((0x72, false)), "f4" => return Some((0x73, false)),
            "f5" => return Some((0x74, false)), "f6" => return Some((0x75, false)),
            "f7" => return Some((0x76, false)), "f8" => return Some((0x77, false)),
            "f9" => return Some((0x78, false)), "f10" => return Some((0x79, false)),
            "f11" => return Some((0x7A, false)), "f12" => return Some((0x7B, false)),
            "shift" | "shift_l" => return Some((0x10, false)),
            "shift_r" => return Some((0xA1, false)),
            "control" | "ctrl" | "control_l" => return Some((0x11, false)),
            "control_r" => return Some((0xA3, false)),
            "alt" | "alt_l" => return Some((0x12, false)),
            "alt_r" => return Some((0xA5, false)),
            "super_l" | "meta_l" => return Some((0x5B, true)),
            "super_r" | "meta_r" => return Some((0x5C, true)),
            "caps_lock" => return Some((0x14, false)),
            _ => {}
        }
        if let Some(ch) = k.chars().next() {
            if ch.is_ascii_alphanumeric() {
                return Some((ch.to_ascii_uppercase() as u16, false));
            }
            let m: Option<u16> = match ch {
                '-' => Some(0xBD), '=' => Some(0xBB), '[' => Some(0xDB), ']' => Some(0xDD),
                '\\' => Some(0xDC), ';' => Some(0xBA), '\'' => Some(0xDE), ',' => Some(0xBC),
                '.' => Some(0xBE), '/' => Some(0xBF), '`' => Some(0xC0),
                _ => None,
            };
            if let Some(v) = m { return Some((v, false)); }
        }
        None
    }

    pub async fn send_input(params: &Value) -> String {
        unsafe {
            let (p_si, p_cc) = match (sym("user32.dll", "SendInput"), sym("user32.dll", "SetCursorPos")) {
                (Some(a), Some(b)) => (a, b),
                _ => return "sendinput_unavailable".to_string(),
            };
            let send: unsafe extern "system" fn(UINT, *mut INPUT, c_int) -> UINT = std::mem::transmute(p_si);
            let cursor: unsafe extern "system" fn(c_int, c_int) -> BOOL = std::mem::transmute(p_cc);

            let input_type = params.get("type").and_then(|v| v.as_str()).unwrap_or("");
            let mk_mouse = |flags: DWORD, dx: i32, dy: i32, data: DWORD| INPUT {
                itype: 0,
                u: INPUT_U { mi: MOUSEINPUT { dx, dy, data, flags, time: 0, extra: 0 } },
            };
            let mk_key = |vk: u16, flags: DWORD| INPUT {
                itype: 1,
                u: INPUT_U { ki: KEYBDINPUT { vk, scan: 0, flags, time: 0, extra: 0 } },
            };
            let isz = std::mem::size_of::<INPUT>() as c_int;
            let mut inputs = [empty_input(); 8];

            match input_type {
                "mouse_move" => {
                    let x = params.get("x").and_then(|v| v.as_i64()).unwrap_or(0) as c_int;
                    let y = params.get("y").and_then(|v| v.as_i64()).unwrap_or(0) as c_int;
                    if cursor(x, y) != 0 { "ok".to_string() } else { "cursor_error".to_string() }
                }
                "mouse_click" => {
                    let x = params.get("x").and_then(|v| v.as_i64()).unwrap_or(0) as c_int;
                    let y = params.get("y").and_then(|v| v.as_i64()).unwrap_or(0) as c_int;
                    let button = params.get("button").and_then(|v| v.as_u64()).unwrap_or(1);
                    cursor(x, y);
                    let (down, up): (DWORD, DWORD) = match button {
                        2 => (0x0008, 0x0010), // RIGHT
                        3 => (0x0020, 0x0040), // MIDDLE
                        _ => (0x0002, 0x0004), // LEFT
                    };
                    inputs[0] = mk_mouse(down, 0, 0, 0);
                    inputs[1] = mk_mouse(up, 0, 0, 0);
                    if send(2, inputs.as_mut_ptr(), isz) == 2 { "ok".to_string() } else { "sendinput_error".to_string() }
                }
                "mouse_down" | "mouse_up" => {
                    let button = params.get("button").and_then(|v| v.as_u64()).unwrap_or(1);
                    let flag = if input_type == "mouse_down" {
                        match button { 2 => 0x0008, 3 => 0x0020, _ => 0x0002 }
                    } else {
                        match button { 2 => 0x0010, 3 => 0x0040, _ => 0x0004 }
                    };
                    inputs[0] = mk_mouse(flag, 0, 0, 0);
                    if send(1, inputs.as_mut_ptr(), isz) == 1 { "ok".to_string() } else { "sendinput_error".to_string() }
                }
                "key" => {
                    let key = params.get("key").and_then(|v| v.as_str()).unwrap_or("");
                    if key.is_empty() { return "error_no_key".to_string(); }
                    // 支持 ctrl+alt+f1 风格组合
                    let mut n = 0usize;
                    let mut ups: Vec<INPUT> = Vec::new();
                    for part in key.split('+') {
                        let p = part.trim();
                        if p.is_empty() { continue; }
                        match vk_from_name(p) {
                            Some((vk, ext)) => {
                                let flags = if ext { 0x0001 } else { 0 };
                                if n < 4 { inputs[n] = mk_key(vk, flags); n += 1; }
                                ups.push(mk_key(vk, flags | 0x0002 /*KEYUP*/));
                            }
                            None => return format!("unknown_key:{}", p),
                        }
                    }
                    if n == 0 { return "error_no_key".to_string(); }
                    for (i, u) in ups.iter().enumerate() {
                        if n + i < 8 { inputs[n + i] = *u; }
                    }
                    let total = (n + ups.len()).min(8) as UINT;
                    if send(total, inputs.as_mut_ptr(), isz) == total { "ok".to_string() } else { "sendinput_error".to_string() }
                }
                "type" => {
                    let text = params.get("text").and_then(|v| v.as_str()).unwrap_or("");
                    if text.is_empty() { return "error_no_text".to_string(); }
                    // Unicode 注入（KEYEVENTF_UNICODE），支持中文
                    let chars: Vec<u16> = text.encode_utf16().collect();
                    let mut sent = 0u32;
                    for &c in &chars {
                        let mut buf = [
                            INPUT { itype: 1, u: INPUT_U { ki: KEYBDINPUT { vk: 0, scan: c, flags: 0x0004, time: 0, extra: 0 } } },
                            INPUT { itype: 1, u: INPUT_U { ki: KEYBDINPUT { vk: 0, scan: c, flags: 0x0004 | 0x0002, time: 0, extra: 0 } } },
                        ];
                        sent += send(2, buf.as_mut_ptr(), isz);
                    }
                    if sent == chars.len() as u32 * 2 { "ok".to_string() } else { format!("type_partial:{}/{}", sent, chars.len() * 2) }
                }
                "scroll" => {
                    let dir = params.get("direction").and_then(|v| v.as_str()).unwrap_or("down");
                    let delta: i32 = if dir == "up" { 120 } else { -120 };
                    inputs[0] = mk_mouse(0x0800 /*WHEEL*/, 0, 0, delta as u32);
                    if send(1, inputs.as_mut_ptr(), isz) == 1 { "ok".to_string() } else { "sendinput_error".to_string() }
                }
                _ => format!("unknown_input_type:{}", input_type),
            }
        }
    }

    // ---------- 活动窗口（考试防切屏用） ----------
    pub fn get_active_window_sync() -> String {
        unsafe {
            if let Some(p) = sym("user32.dll", "GetForegroundWindow") {
                let f: unsafe extern "system" fn() -> HWND = std::mem::transmute(p);
                let hwnd = f();
                if !hwnd.is_null() {
                    return format!("{:#x}", hwnd as usize);
                }
            }
        }
        String::new()
    }

    pub fn get_window_name_sync(window_id: &str) -> String {
        unsafe {
            let hwnd = match parse_hwnd(window_id) { Some(h) => h, None => return String::new() };
            if let Some(p) = sym("user32.dll", "GetWindowTextW") {
                let f: unsafe extern "system" fn(HWND, *mut WCHAR, c_int) -> c_int = std::mem::transmute(p);
                let mut buf = vec![0u16; 512];
                let n = f(hwnd, buf.as_mut_ptr(), 512);
                if n > 0 {
                    return String::from_utf16_lossy(&buf[..n as usize]);
                }
            }
        }
        String::new()
    }

    pub fn activate_window_sync(window_id: &str) -> bool {
        unsafe {
            let hwnd = match parse_hwnd(window_id) { Some(h) => h, None => return false };
            let mut ok = false;
            if let Some(p) = sym("user32.dll", "ShowWindow") {
                let f: unsafe extern "system" fn(HWND, c_int) -> BOOL = std::mem::transmute(p);
                f(hwnd, 9 /*SW_RESTORE*/);
            }
            if let Some(p) = sym("user32.dll", "SetForegroundWindow") {
                let f: unsafe extern "system" fn(HWND) -> BOOL = std::mem::transmute(p);
                ok = f(hwnd) != 0;
            }
            ok
        }
    }

    fn parse_hwnd(id: &str) -> Option<HWND> {
        let t = id.trim();
        let v = if let Some(hex) = t.strip_prefix("0x").or_else(|| t.strip_prefix("0X")) {
            usize::from_str_radix(hex, 16).ok()?
        } else {
            t.parse::<usize>().ok()?
        };
        if v == 0 { None } else { Some(v as HWND) }
    }

    // ---------- 电源 ----------
    pub fn execute_power(reboot: bool) {
        if std::env::var("LAB_POWER_DRYRUN").map(|v| v == "1").unwrap_or(false) {
            println!("[Agent] DRYRUN: 跳过真实{}", if reboot { "重启" } else { "关机" });
            return;
        }
        unsafe {
            enable_shutdown_privilege();
            if let Some(p) = sym("advapi32.dll", "InitiateSystemShutdownExW") {
                let f: unsafe extern "system" fn(*const WCHAR, *const WCHAR, DWORD, BOOL, BOOL, DWORD) -> BOOL =
                    std::mem::transmute(p);
                let msg = to_wide("机房教学管理系统：教师发起电源操作");
                let r = f(std::ptr::null(), msg.as_ptr(), 0 /*调用方已延时*/,
                    1 /*强制关闭应用*/, if reboot { 1 } else { 0 },
                    0x8000_0000 /*PLANNED*/);
                if r != 0 {
                    println!("[Agent] InitiateSystemShutdownEx 已执行: reboot={}", reboot);
                    return;
                }
            }
        }
        // 回退：shutdown.exe
        let args: Vec<String> = if reboot {
            vec!["/r".into(), "/t".into(), "0".into(), "/f".into()]
        } else {
            vec!["/s".into(), "/t".into(), "0".into(), "/f".into()]
        };
        match std::process::Command::new("shutdown.exe").args(&args).output() {
            Ok(o) if o.status.success() => println!("[Agent] shutdown.exe 已执行"),
            Ok(o) => println!("[Agent] shutdown.exe 失败: {}", String::from_utf8_lossy(&o.stderr).trim()),
            Err(e) => println!("[Agent] shutdown.exe 不可用: {}", e),
        }
    }

    unsafe fn enable_shutdown_privilege() {
        let (p_ot, p_lpv, p_atp) = match (
            sym("advapi32.dll", "OpenProcessToken"),
            sym("advapi32.dll", "LookupPrivilegeValueW"),
            sym("advapi32.dll", "AdjustTokenPrivileges"),
        ) {
            (Some(a), Some(b), Some(c)) => (a, b, c),
            _ => return,
        };
        #[repr(C)]
        struct LUID { low: u32, high: i32 }
        #[repr(C)]
        struct LUID_AND_ATTR { luid: LUID, attr: u32 }
        #[repr(C)]
        struct TOKEN_PRIVS { count: u32, privs: [LUID_AND_ATTR; 1] }

        let open_token: unsafe extern "system" fn(HANDLE, DWORD, *mut HANDLE) -> BOOL = std::mem::transmute(p_ot);
        let lookup: unsafe extern "system" fn(*const WCHAR, *const WCHAR, *mut LUID) -> BOOL = std::mem::transmute(p_lpv);
        let adjust: unsafe extern "system" fn(HANDLE, BOOL, *mut TOKEN_PRIVS, DWORD, *mut c_void, *mut u32) -> BOOL = std::mem::transmute(p_atp);

        let mut token: HANDLE = std::ptr::null_mut();
        let cur_proc = (-1isize) as HANDLE; // GetCurrentProcess() 伪句柄
        if open_token(cur_proc, 0x0028 /*QUERY|ADJUST*/, &mut token) == 0 { return; }
        let name = to_wide("SeShutdownPrivilege");
        let mut luid = LUID { low: 0, high: 0 };
        if lookup(std::ptr::null(), name.as_ptr(), &mut luid) != 0 {
            let mut tp = TOKEN_PRIVS {
                count: 1,
                privs: [LUID_AND_ATTR { luid, attr: 0x0000_0002 /*ENABLED*/ }],
            };
            adjust(token, 0, &mut tp, 0, std::ptr::null_mut(), std::ptr::null_mut());
        }
    }

    // ---------- 会话注销 ----------
    pub async fn session_logoff(user: &str, mode: &str) -> Result<String, String> {
        if !user.is_empty() {
            println!("[Agent] Windows 注销忽略指定用户参数: {}（注销当前控制台会话）", user);
        }
        unsafe {
            // SYSTEM 服务路径：WTSQueryUserToken + winlogoff.exe
            let sid = if let Some(p) = sym("kernel32.dll", "WTSGetActiveConsoleSessionId") {
                let f: unsafe extern "system" fn() -> DWORD = std::mem::transmute(p);
                f()
            } else {
                0xFFFF_FFFF
            };
            if sid != 0xFFFF_FFFF {
                if let Some(p_q) = sym("wtsapi32.dll", "WTSQueryUserToken") {
                    let q: unsafe extern "system" fn(DWORD, *mut HANDLE) -> BOOL = std::mem::transmute(p_q);
                    let mut utoken: HANDLE = std::ptr::null_mut();
                    if q(sid, &mut utoken) != 0 && !utoken.is_null() {
                        if let Some(msg) = run_winlogoff(utoken) {
                            return Ok(msg);
                        }
                    }
                }
            }
            // 回退：ExitWindowsEx（交互用户会话内运行时有效）
            enable_shutdown_privilege();
            if let Some(p) = sym("user32.dll", "ExitWindowsEx") {
                let f: unsafe extern "system" fn(UINT, DWORD) -> BOOL = std::mem::transmute(p);
                let flags: UINT = if mode == "kill" { 0x0004 /*EWX_FORCE*/ } else { 0 } | 0 /*EWX_LOGOFF*/;
                if f(flags, 0) != 0 {
                    return Ok("已发起会话注销".to_string());
                }
                return Err(format!("会话注销失败(GetLastError={})", GetLastError()));
            }
            Err("ExitWindowsEx 不可用".to_string())
        }
    }

    /// 用控制台用户 token 启动 winlogoff.exe（Agent 以 SYSTEM 运行时）
    unsafe fn run_winlogoff(utoken: HANDLE) -> Option<String> {
        let p_dup = sym("advapi32.dll", "DuplicateTokenEx")?;
        let p_cpw = sym("advapi32.dll", "CreateProcessWithTokenW")?;
        let p_gsd = sym("kernel32.dll", "GetSystemDirectoryW")?;

        #[repr(C)]
        struct STARTUPINFOW {
            cb: u32, reserved: *mut WCHAR, desktop: *mut WCHAR, title: *mut WCHAR,
            x: DWORD, y: DWORD, xsize: DWORD, ysize: DWORD, xchars: DWORD, ychars: DWORD,
            fill: DWORD, flags: DWORD, show: u16, cb2: u16, reserved2: *mut u8,
            stdin: HANDLE, stdout: HANDLE, stderr: HANDLE,
        }
        #[repr(C)]
        struct PROCESS_INFO { process: HANDLE, thread: HANDLE, pid: DWORD, tid: DWORD }

        let dup: unsafe extern "system" fn(HANDLE, DWORD, *mut c_void, u32, u32, *mut HANDLE) -> BOOL =
            std::mem::transmute(p_dup);
        let cpw: unsafe extern "system" fn(HANDLE, DWORD, *mut WCHAR, DWORD, *mut c_void, *mut c_void, *mut WCHAR, *mut STARTUPINFOW, *mut PROCESS_INFO) -> BOOL =
            std::mem::transmute(p_cpw);
        let gsd: unsafe extern "system" fn(*mut WCHAR, UINT) -> UINT = std::mem::transmute(p_gsd);

        let mut primary: HANDLE = std::ptr::null_mut();
        if dup(utoken, 0 /*MAXIMUM_ALLOWED*/, std::ptr::null_mut(), 2, 1, &mut primary) == 0 || primary.is_null() {
            return None;
        }
        let mut dir = vec![0u16; 512];
        let n = gsd(dir.as_mut_ptr(), 512);
        let sysdir = String::from_utf16_lossy(&dir[..n as usize]);
        let mut cmdline = to_wide(&format!("{}\\winlogoff.exe", sysdir));
        let mut si: STARTUPINFOW = std::mem::zeroed();
        si.cb = std::mem::size_of::<STARTUPINFOW>() as u32;
        let mut pi: PROCESS_INFO = std::mem::zeroed();
        let r = cpw(primary, 0, cmdline.as_mut_ptr(), 0, std::ptr::null_mut(), std::ptr::null_mut(),
            std::ptr::null_mut(), &mut si, &mut pi);
        if r != 0 {
            Some("已通过 winlogoff 注销会话".to_string())
        } else {
            None
        }
    }

    // ---------- Shell / 打开 ----------
    pub async fn run_shell(cmd: &str) -> Result<std::process::Output, String> {
        tokio::process::Command::new("cmd.exe").args(["/C", cmd]).output().await
            .map_err(|e| format!("启动失败: {}", e))
    }

    pub fn open_target(typ: &str, target: &str) -> Result<String, String> {
        if typ == "url" {
            unsafe {
                if let Some(p) = sym("shell32.dll", "ShellExecuteW") {
                    let f: unsafe extern "system" fn(HWND, *const WCHAR, *const WCHAR, *const WCHAR, *const WCHAR, c_int) -> HANDLE =
                        std::mem::transmute(p);
                    let op = to_wide("open");
                    let t = to_wide(target);
                    let r = f(std::ptr::null_mut(), op.as_ptr(), t.as_ptr(), std::ptr::null(), std::ptr::null(), 5 /*SW_SHOWNORMAL*/);
                    if r as usize > 32 {
                        return Ok(format!("已打开网址: {}", target));
                    }
                    return Err(format!("ShellExecute 失败: {}", r as usize));
                }
            }
            return Err("shell32 不可用".to_string());
        }
        // app：CreateProcessW 启动
        unsafe {
            let p_cp = sym("kernel32.dll", "CreateProcessW")
                .ok_or("CreateProcessW 不可用")?;
            #[repr(C)]
            struct STARTUPINFOW {
                cb: u32, reserved: *mut WCHAR, desktop: *mut WCHAR, title: *mut WCHAR,
                x: DWORD, y: DWORD, xsize: DWORD, ysize: DWORD, xchars: DWORD, ychars: DWORD,
                fill: DWORD, flags: DWORD, show: u16, cb2: u16, reserved2: *mut u8,
                stdin: HANDLE, stdout: HANDLE, stderr: HANDLE,
            }
            #[repr(C)]
            struct PROCESS_INFO { process: HANDLE, thread: HANDLE, pid: DWORD, tid: DWORD }
            let cp: unsafe extern "system" fn(*const WCHAR, *mut WCHAR, *mut c_void, *mut c_void, BOOL, DWORD, *mut c_void, *const WCHAR, *mut STARTUPINFOW, *mut PROCESS_INFO) -> BOOL =
                std::mem::transmute(p_cp);
            let mut cmdline = to_wide(target);
            let mut si: STARTUPINFOW = std::mem::zeroed();
            si.cb = std::mem::size_of::<STARTUPINFOW>() as u32;
            let mut pi: PROCESS_INFO = std::mem::zeroed();
            if cp(std::ptr::null(), cmdline.as_mut_ptr(), std::ptr::null_mut(), std::ptr::null_mut(),
                0, 0, std::ptr::null_mut(), std::ptr::null(), &mut si, &mut pi) != 0 {
                Ok(format!("已启动: {}", target))
            } else {
                Err(format!("启动失败(GetLastError={})", GetLastError()))
            }
        }
    }

    /// Windows 不允许覆盖运行中的 exe：先把运行中的 exe 改名 .old，再移入新文件
    pub fn replace_running_exe(tmp: &str, exe: &str) -> Result<(), String> {
        let old = format!("{}.old", exe);
        std::fs::remove_file(&old).ok();
        std::fs::rename(exe, &old).map_err(|e| format!("移走旧二进制失败: {}", e))?;
        if let Err(e) = std::fs::rename(tmp, exe) {
            // 尽力恢复
            std::fs::rename(&old, exe).ok();
            return Err(format!("替换二进制失败: {}", e));
        }
        Ok(())
    }
}

#[cfg(target_os = "windows")]
pub use imp::*;
