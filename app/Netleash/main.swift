// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Netleash contributors
// Netleash — the single app of the project: launcher for the protected app, menu bar status
// icon, leak test, log viewer and emergency rollback. It makes no network decisions itself: it only reads the
// daemon's status file and starts the protected app through the setuid helper.
//
//   Netleash              open the protected app (and show the icon if it is not shown yet)
//   Netleash --background only the icon (this is how it starts at login)

import AppKit
import UserNotifications

let installName = "netleash"
let statusPath = "/var/run/\(installName)/status.json"
let libexec = "/usr/local/libexec/\(installName)"
let helperPath = "\(libexec)/netleash-exec"
let cliPath = "/usr/local/bin/netleash"
let logPath = "/var/log/\(installName).log"
let protectedBinary = "/Applications/Claude.app/Contents/MacOS/Claude"
let protectedBundleID = "com.anthropic.claudefordesktop"
let agentLabel = "org.netleash.app"
let agentPath = NSHomeDirectory() + "/Library/LaunchAgents/\(agentLabel).plist"
let updateDailyKey = "updateCheckDaily"
let lastCheckKey = "lastUpdateCheck"
let notifiedKey = "notifiedUpdateVersion"
let launchLog = NSHomeDirectory() + "/Library/Logs/Netleash-launch.log"

/// Localized string: the English text is the key (see Resources/ru.lproj/Localizable.strings).
func L(_ key: String, _ args: CVarArg...) -> String {
    let f = NSLocalizedString(key, comment: "")
    return args.isEmpty ? f : String(format: f, arguments: args)
}

// MARK: - Protection state

enum State {
    case up(String)      // VPN is up, protection armed
    case down            // VPN is down, network of the protected app is blocked
    case broken(String)  // daemon not running / pf not armed

    var symbol: String {   // SF Symbol, used only as a fallback if the bundled glyph is missing
        switch self {
        case .up: return "lock.shield.fill"
        case .down: return "lock.slash.fill"
        case .broken: return "exclamationmark.triangle.fill"
        }
    }
    var glyph: String {    // bundled menu bar glyph (Resources/menubar-<glyph>@2x.png, drawn at 18 pt)
        switch self {
        case .up: return "up"
        case .down: return "down"
        case .broken: return "warn"
        }
    }
    var color: NSColor {
        switch self {
        case .up: return .systemGreen
        case .down: return .systemRed
        case .broken: return .systemOrange
        }
    }
    var title: String {
        switch self {
        case .up(let i): return L("VPN connected (%@) — network allowed", i)
        case .down: return L("VPN disconnected — network blocked")
        case .broken(let why): return L("Protection is not working: %@", why)
        }
    }
}

/// Events published by the daemon in the status file (VPN down/up, unprotected launch closed).
var lastEventSeq: Int64?

func handleEvents() {
    guard let data = FileManager.default.contents(atPath: statusPath),
          let d = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
          let ev = d["event"] as? [String: Any], let seq = (ev["seq"] as? NSNumber)?.int64Value else { return }
    guard let last = lastEventSeq else { lastEventSeq = seq; return }   // first read: do not replay old events
    guard seq > last else { return }
    lastEventSeq = seq
    var title = ev["title"] as? String ?? "Netleash"
    var text = ev["text"] as? String ?? ""
    switch ev["kind"] as? String {   // known kinds are shown in the user's language
    case "vpn_down": title = L("VPN disconnected"); text = L("Network access is blocked.")
    case "vpn_up": title = L("VPN connected"); text = L("Network access restored through the VPN.")
    case "app_closed": title = L("App closed"); text = L("Start it only through Netleash.")
    default: break
    }
    Notifier.shared.post(title, text)
}

func readState() -> State {
    guard let data = FileManager.default.contents(atPath: statusPath),
          let d = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
          let ts = d["ts"] as? Double else { return .broken(L("daemon is not running")) }
    if Date().timeIntervalSince1970 - ts > 5 { return .broken(L("daemon is not responding")) }
    if (d["armed"] as? Bool) != true { return .broken((d["note"] as? String) ?? L("pf is not armed")) }
    return (d["vpnUp"] as? Bool) == true ? .up((d["iface"] as? String) ?? "?") : .down
}

// MARK: - Helpers

/// Runs a command as root through the standard macOS administrator prompt.
func runAsAdmin(_ cmd: String) -> String {
    var err: NSDictionary?
    let r = NSAppleScript(source: "do shell script \"\(cmd)\" with administrator privileges")?.executeAndReturnError(&err)
    if let e = err { return L("Error: %@", "\(e[NSAppleScript.errorMessage] ?? e)") }
    return r?.stringValue ?? L("Done")
}

func alert(_ title: String, _ text: String = "", style: NSAlert.Style = .informational) {
    let a = NSAlert()
    a.messageText = title
    a.informativeText = text
    a.alertStyle = style
    NSApp.activate(ignoringOtherApps: true)
    a.runModal()
}

func launchctl(_ args: [String]) {
    let p = Process()
    p.executableURL = URL(fileURLWithPath: "/bin/launchctl")
    p.arguments = args
    p.standardOutput = FileHandle.nullDevice
    p.standardError = FileHandle.nullDevice
    try? p.run()
    p.waitUntilExit()
}

// MARK: - Toast: Netleash's own notification banner (used when macOS does not allow notifications for the app)

final class Toast {
    static var panel: NSPanel?
    static var generation = 0

    static func content(_ title: String, _ body: String, _ w: CGFloat = 360, _ h: CGFloat = 76) -> NSVisualEffectView {
        let fx = NSVisualEffectView(frame: NSRect(x: 0, y: 0, width: w, height: h))
            fx.material = .hudWindow
            fx.state = .active
            fx.wantsLayer = true
            fx.layer?.cornerRadius = 16
            fx.layer?.masksToBounds = true

            let icon = NSImageView(frame: NSRect(x: 14, y: (h - 44) / 2, width: 44, height: 44))
            icon.image = Bundle.main.path(forResource: "icon", ofType: "icns").flatMap { NSImage(contentsOfFile: $0) } ?? NSApp.applicationIconImage
            let t = NSTextField(labelWithString: title)
            t.font = .boldSystemFont(ofSize: 13)
            t.frame = NSRect(x: 70, y: h - 34, width: w - 84, height: 18)
            let b = NSTextField(wrappingLabelWithString: body)
            b.font = .systemFont(ofSize: 12)
            b.textColor = .secondaryLabelColor
            b.maximumNumberOfLines = 2
            b.frame = NSRect(x: 70, y: 10, width: w - 84, height: h - 46)
            [icon, t, b].forEach { fx.addSubview($0) }
        return fx
    }

    static func show(_ title: String, _ body: String, seconds: Double = 7) {
        DispatchQueue.main.async {
            panel?.orderOut(nil)
            let w: CGFloat = 360, h: CGFloat = 76
            let p = NSPanel(contentRect: NSRect(x: 0, y: 0, width: w, height: h),
                            styleMask: [.borderless, .nonactivatingPanel], backing: .buffered, defer: false)
            p.isFloatingPanel = true
            p.level = .statusBar
            p.hasShadow = true
            p.isOpaque = false
            p.backgroundColor = .clear
            p.isReleasedWhenClosed = false
            p.hidesOnDeactivate = false
            p.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .stationary]

            let fx = content(title, body, w, h)
            p.contentView = fx

            if let scr = NSScreen.main {
                let f = scr.visibleFrame
                p.setFrameOrigin(NSPoint(x: f.maxX - w - 12, y: f.maxY - h - 12))
            }
            p.alphaValue = 0
            p.orderFrontRegardless()
            NSAnimationContext.runAnimationGroup { $0.duration = 0.2; p.animator().alphaValue = 1 }
            panel = p
            generation += 1
            let mine = generation
            DispatchQueue.main.asyncAfter(deadline: .now() + seconds) {
                guard generation == mine, let cur = panel else { return }
                NSAnimationContext.runAnimationGroup({ $0.duration = 0.4; cur.animator().alphaValue = 0 },
                                                     completionHandler: { if generation == mine { cur.orderOut(nil) } })
            }
        }
    }
}

// MARK: - Notifications (shown under the Netleash name and icon)

final class Notifier: NSObject, UNUserNotificationCenterDelegate {
    static let shared = Notifier()
    /// UNUserNotificationCenter needs a real app bundle; a bare binary falls back to AppleScript.
    private var usable: Bool { Bundle.main.bundlePath.hasSuffix(".app") }

    /// Diagnostics (~/Library/Logs/Netleash.log): why a notification did or did not appear.
    func log(_ s: String) {
        let path = NSHomeDirectory() + "/Library/Logs/Netleash.log"
        let line = "[\(ISO8601DateFormatter().string(from: Date()))] \(s)\n"
        if let h = FileHandle(forWritingAtPath: path) { h.seekToEndOfFile(); h.write(Data(line.utf8)); h.closeFile() }
        else { try? line.write(toFile: path, atomically: true, encoding: .utf8) }
    }

    func setup() {
        guard usable else { log("not running from an app bundle: notifications use Netleash banners"); return }
        let c = UNUserNotificationCenter.current()
        c.delegate = self
        c.requestAuthorization(options: [.alert, .sound]) { granted, err in
            self.log("authorization granted=\(granted) error=\(err.map { "\($0)" } ?? "none")")
        }
        c.getNotificationSettings { s in
            self.log("settings: authorizationStatus=\(s.authorizationStatus.rawValue) (2 = authorized) alert=\(s.alertSetting.rawValue)")
        }
    }

    /// Used when macOS does not allow notifications for Netleash (typical for ad-hoc signed apps): Netleash shows its own banner.
    private func fallback(_ title: String, _ body: String) { Toast.show(title, body) }

    func post(_ title: String, _ body: String) {
        guard usable else { fallback(title, body); return }
        let center = UNUserNotificationCenter.current()
        center.getNotificationSettings { s in
            guard s.authorizationStatus == .authorized || s.authorizationStatus == .provisional else {
                self.log("macOS notifications not allowed (status \(s.authorizationStatus.rawValue)); showing a Netleash banner for: \(title)")
                self.fallback(title, body)
                return
            }
            let content = UNMutableNotificationContent()
            content.title = title
            content.body = body
            center.add(UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil)) { err in
                if let err = err { self.log("posting failed: \(err); using the fallback"); self.fallback(title, body) }
            }
        }
    }

    func userNotificationCenter(_ center: UNUserNotificationCenter, willPresent notification: UNNotification,
                                withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void) {
        completionHandler([.banner, .list, .sound])
    }
}

/// Starts a program that macOS treats as its OWN "responsible process". With a plain Process the protected app would be
/// attributed to Netleash, so privacy prompts (Downloads, Documents, Desktop, …) would name Netleash instead of the app,
/// and permissions already granted to the app (screen recording, accessibility, …) would not apply. Uses the same
/// call that terminals and launchers use to disclaim responsibility for what they spawn.
func spawnDisclaimed(_ path: String, _ args: [String]) -> pid_t? {
    FileManager.default.createFile(atPath: launchLog, contents: nil)
    var attr: posix_spawnattr_t?
    posix_spawnattr_init(&attr)
    defer { posix_spawnattr_destroy(&attr) }
    typealias Disclaim = @convention(c) (UnsafeMutablePointer<posix_spawnattr_t?>, Int32) -> Int32
    if let sym = dlsym(UnsafeMutableRawPointer(bitPattern: -2), "responsibility_spawnattrs_setdisclaim") {   // RTLD_DEFAULT
        _ = unsafeBitCast(sym, to: Disclaim.self)(&attr, 1)
    }
    var actions: posix_spawn_file_actions_t?
    posix_spawn_file_actions_init(&actions)
    defer { posix_spawn_file_actions_destroy(&actions) }
    posix_spawn_file_actions_addopen(&actions, 0, "/dev/null", O_RDONLY, 0)
    posix_spawn_file_actions_addopen(&actions, 1, "/dev/null", O_WRONLY, 0)
    posix_spawn_file_actions_addopen(&actions, 2, launchLog, O_WRONLY | O_APPEND, 0o600)

    let argv: [UnsafeMutablePointer<CChar>?] = ([path] + args).map { strdup($0) } + [nil]
    defer { argv.forEach { free($0) } }
    var pid: pid_t = 0
    let rc = posix_spawn(&pid, path, &actions, &attr, argv, environ)
    return rc == 0 ? pid : nil
}

// MARK: - App

final class App: NSObject, NSApplicationDelegate {
    let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
    let header = NSMenuItem(title: "", action: nil, keyEquivalent: "")
    let loginItem = NSMenuItem(title: L("Start at login"), action: nil, keyEquivalent: "")
    let testItem = NSMenuItem(title: L("Run leak test…"), action: nil, keyEquivalent: "")
    let updateItem = NSMenuItem(title: L("Check for updates…"), action: nil, keyEquivalent: "")
    let dailyItem = NSMenuItem(title: L("Check for updates daily (through the VPN)"), action: nil, keyEquivalent: "")
    var availableUpdate: String?   // set when a newer release is known
    var last = ""
    let background = CommandLine.arguments.contains("--background")

    func applicationDidFinishLaunching(_ n: Notification) {
        Notifier.shared.setup()
        let menu = NSMenu()
        header.isEnabled = false
        menu.addItem(header)
        let ver = NSMenuItem(title: "Netleash \(Bundle.main.object(forInfoDictionaryKey: "CFBundleVersion") as? String ?? "?")", action: nil, keyEquivalent: "")
        ver.isEnabled = false
        menu.addItem(ver)
        menu.addItem(.separator())
        add(menu, L("Open Claude"), #selector(openProtectedApp))
        menu.addItem(.separator())
        testItem.action = #selector(testLeaks); testItem.target = self; menu.addItem(testItem)
        add(menu, L("Show log"), #selector(showLog))
        add(menu, L("Notification settings…"), #selector(openNotificationSettings))
        menu.addItem(.separator())
        updateItem.action = #selector(checkForUpdatesAction); updateItem.target = self; menu.addItem(updateItem)
        dailyItem.action = #selector(toggleDaily); dailyItem.target = self; menu.addItem(dailyItem)
        menu.addItem(.separator())
        loginItem.action = #selector(toggleLogin); loginItem.target = self; menu.addItem(loginItem)
        menu.addItem(.separator())
        add(menu, L("Emergency rollback (remove protection)…"), #selector(panic))
        add(menu, L("Restore protection"), #selector(resume))
        menu.addItem(.separator())
        add(menu, L("Quit"), #selector(quit), key: "q")
        menu.delegate = self
        item.menu = menu
        refresh()
        Timer.scheduledTimer(withTimeInterval: 0.5, repeats: true) { [weak self] _ in self?.refresh() }
        Timer.scheduledTimer(withTimeInterval: 3600, repeats: true) { [weak self] _ in self?.dailyCheckIfDue() }
        DispatchQueue.main.asyncAfter(deadline: .now() + 60) { [weak self] in self?.dailyCheckIfDue() }
        if !background { openProtectedApp() }
    }

    /// The app was opened again (Dock, Spotlight, double click) while the icon is already running.
    func applicationShouldHandleReopen(_ app: NSApplication, hasVisibleWindows: Bool) -> Bool {
        openProtectedApp()
        return false
    }

    func add(_ menu: NSMenu, _ title: String, _ sel: Selector, key: String = "") {
        let i = NSMenuItem(title: title, action: sel, keyEquivalent: key)
        i.target = self
        menu.addItem(i)
    }

    func refresh() {
        handleEvents()
        let s = readState()
        header.title = s.title
        item.button?.toolTip = s.title
        let key = s.symbol + s.title
        guard key != last else { return }
        last = key
        var img: NSImage?
        if let path = Bundle.main.path(forResource: "menubar-\(s.glyph)@2x", ofType: "png"), let g = NSImage(contentsOfFile: path) {
            g.size = NSSize(width: 18, height: 18)   // the file is 36 px: 2x of 18 pt
            img = g
        } else {
            img = NSImage(systemSymbolName: s.symbol, accessibilityDescription: s.title)?
                .withSymbolConfiguration(NSImage.SymbolConfiguration(paletteColors: [s.color]))
        }
        img?.isTemplate = false
        item.button?.image = img
    }

    // MARK: Actions

    /// Starts the protected app under protection (or brings an already running one to the front).
    @objc func openProtectedApp() {
        if let r = NSRunningApplication.runningApplications(withBundleIdentifier: protectedBundleID).first {
            r.activate()
            return
        }
        guard let pid = spawnDisclaimed(helperPath, [protectedBinary]) else {
            alert(L("Could not start the app"), L("Check the installation: netleash status"), style: .critical)
            return
        }
        // The helper refuses to start when protection is not armed: say so instead of failing silently.
        let started = Date()
        DispatchQueue.global().async {
            var status: Int32 = 0
            waitpid(pid, &status, 0)
            let failedFast = Date().timeIntervalSince(started) < 5
            let code = (status & 0x7f) == 0 ? (status >> 8) & 0xff : 128 + (status & 0x7f)
            guard failedFast, code != 0 else { return }
            DispatchQueue.main.async {
                let msg = (try? String(contentsOfFile: launchLog, encoding: .utf8)) ?? ""
                alert(L("Not started: protection is not active"), msg.isEmpty ? L("Check: netleash status") : msg, style: .critical)
            }
        }
    }

    @objc func testLeaks() {
        testItem.title = L("Testing…")
        testItem.isEnabled = false
        DispatchQueue.global().async {
            let p = Process()
            p.executableURL = URL(fileURLWithPath: cliPath)
            p.arguments = ["test"]
            let out = Pipe(); p.standardOutput = out; p.standardError = out
            try? p.run()
            let data = out.fileHandleForReading.readDataToEndOfFile()
            p.waitUntilExit()
            DispatchQueue.main.async {
                self.testItem.title = L("Run leak test…")
                self.testItem.isEnabled = true
                alert(p.terminationStatus == 0 ? L("No leaks found") : L("Leak test failed"),
                      String(decoding: data, as: UTF8.self), style: p.terminationStatus == 0 ? .informational : .critical)
            }
        }
    }

    @objc func openNotificationSettings() {
        NSWorkspace.shared.open(URL(string: "x-apple.systempreferences:com.apple.Notifications-Settings.extension")!)
    }
    @objc func showLog() { NSWorkspace.shared.open(URL(fileURLWithPath: logPath)) }
    @objc func quit() { NSApp.terminate(nil) }

    @objc func panic() {
        let a = NSAlert()
        a.messageText = L("Remove all protection?")
        a.informativeText = L("The daemon will be stopped and the pf rules removed. The protected app will be able to use the network without the VPN until you choose “Restore protection”.")
        a.alertStyle = .warning
        a.addButton(withTitle: L("Cancel"))
        a.addButton(withTitle: L("Roll back"))
        NSApp.activate(ignoringOtherApps: true)
        guard a.runModal() == .alertSecondButtonReturn else { return }
        alert(runAsAdmin("\(libexec)/panic.sh"))
    }

    @objc func resume() { alert(runAsAdmin("\(libexec)/resume.sh")) }

    // MARK: Updates

    /// Runs `netleash update …` (through the VPN unless the user asked otherwise) and returns the parsed JSON.
    func runUpdate(_ args: [String], completion: @escaping ([String: Any]) -> Void) {
        DispatchQueue.global().async {
            let p = Process()
            p.executableURL = URL(fileURLWithPath: cliPath)
            p.arguments = ["update", "--json"] + args
            let out = Pipe(); p.standardOutput = out; p.standardError = FileHandle.nullDevice
            var result: [String: Any] = ["error": "could not run netleash"]
            if (try? p.run()) != nil {
                let data = out.fileHandleForReading.readDataToEndOfFile()
                p.waitUntilExit()
                if let j = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] { result = j }
            }
            DispatchQueue.main.async { completion(result) }
        }
    }

    @objc func checkForUpdatesAction() { checkForUpdates(userInitiated: true) }

    func checkForUpdates(userInitiated: Bool) {
        if userInitiated { updateItem.title = L("Checking…"); updateItem.isEnabled = false }
        runUpdate(["--check"]) { r in
            self.updateItem.title = L("Check for updates…"); self.updateItem.isEnabled = true
            UserDefaults.standard.set(Date().timeIntervalSince1970, forKey: lastCheckKey)
            if let err = r["error"] as? String, !err.isEmpty {
                if userInitiated { alert(L("Could not check for updates"), err, style: .warning) }
                return
            }
            let current = r["current"] as? String ?? "?"
            guard (r["update"] as? Bool) == true, let latest = r["latest"] as? String else {
                self.availableUpdate = nil
                if userInitiated { alert(L("Netleash is up to date"), L("Version %@.", current)) }
                return
            }
            self.availableUpdate = latest
            if userInitiated {
                self.offerUpdate(current: current, latest: latest, url: r["url"] as? String)
            } else if UserDefaults.standard.string(forKey: notifiedKey) != latest {
                UserDefaults.standard.set(latest, forKey: notifiedKey)
                Notifier.shared.post(L("Netleash update"), L("Version %@ is available. Open the Netleash menu to update.", latest))
            }
        }
    }

    func offerUpdate(current: String, latest: String, url: String?) {
        let a = NSAlert()
        a.messageText = L("Netleash %@ is available", latest)
        a.informativeText = L("You have %@. The update is downloaded through the VPN and its SHA-256 checksum is verified; you are asked for your administrator password to install it. Claude is closed during the update and reopened afterwards (save your work first), and this menu bar app restarts.", current)
        a.addButton(withTitle: L("Update"))
        a.addButton(withTitle: L("Release notes"))
        a.addButton(withTitle: L("Later"))
        NSApp.activate(ignoringOtherApps: true)
        switch a.runModal() {
        case .alertFirstButtonReturn: performUpdate()
        case .alertSecondButtonReturn: if let u = url, let link = URL(string: u) { NSWorkspace.shared.open(link) }
        default: break
        }
    }

    func performUpdate() {
        updateItem.title = L("Downloading update…"); updateItem.isEnabled = false
        runUpdate(["--download-only"]) { r in
            self.updateItem.title = L("Check for updates…"); self.updateItem.isEnabled = true
            guard let installer = r["installer"] as? String, let dir = r["dir"] as? String else {
                alert(L("Update failed"), (r["error"] as? String) ?? L("unknown error"), style: .critical)
                return
            }
            // The installer replaces this app and restarts it; the result dialog may not survive that.
            let outcome = runAsAdmin("\(installer) -y")
            try? FileManager.default.removeItem(atPath: dir)
            alert(L("Update finished"), outcome)
        }
    }

    @objc func toggleDaily() {
        let d = UserDefaults.standard
        d.set(!d.bool(forKey: updateDailyKey), forKey: updateDailyKey)
        if d.bool(forKey: updateDailyKey) { checkForUpdates(userInitiated: false) }
    }

    /// Opt-in: at most one silent check per day. The request goes through the VPN; if the VPN is down it is skipped.
    func dailyCheckIfDue() {
        let d = UserDefaults.standard
        guard d.bool(forKey: updateDailyKey) else { return }
        if Date().timeIntervalSince1970 - d.double(forKey: lastCheckKey) < 86400 { return }
        guard case .up = readState() else { return }
        checkForUpdates(userInitiated: false)
    }

    // MARK: Start at login

    var loginEnabled: Bool { FileManager.default.fileExists(atPath: agentPath) }

    @objc func toggleLogin() {
        let uid = String(getuid())
        if loginEnabled {
            launchctl(["bootout", "gui/\(uid)/\(agentLabel)"])
            try? FileManager.default.removeItem(atPath: agentPath)
        } else {
            // Started through LaunchServices ("open"): macOS only registers an app for notifications when it is launched that way.
            let plist: [String: Any] = ["Label": agentLabel,
                                        "ProgramArguments": ["/usr/bin/open", "-g", "-a", Bundle.main.bundlePath, "--args", "--background"],
                                        "RunAtLoad": true]
            try? FileManager.default.createDirectory(atPath: (agentPath as NSString).deletingLastPathComponent, withIntermediateDirectories: true)
            (plist as NSDictionary).write(toFile: agentPath, atomically: true)
            // Not started now: the app is already running, the agent takes effect at the next login.
        }
    }
}

extension App: NSMenuDelegate {
    func menuWillOpen(_ menu: NSMenu) {
        loginItem.state = loginEnabled ? .on : .off
        dailyItem.state = UserDefaults.standard.bool(forKey: updateDailyKey) ? .on : .off
        updateItem.title = availableUpdate.map { L("Update available: %@…", $0) } ?? L("Check for updates…")
    }
}

if CommandLine.arguments.count >= 3, CommandLine.arguments[1] == "--l10n" {
    // Translation check: Netleash --l10n "Quit"   (the language follows the system, or: -AppleLanguages "(ru)")
    print(L(CommandLine.arguments[2]))
    exit(0)
}
if CommandLine.arguments.count >= 4, CommandLine.arguments[1] == "--render-toast" {
    // Layout check without a screen: Netleash --render-toast out.png "Title" "Text"
    _ = NSApplication.shared
    let v = Toast.content(CommandLine.arguments[3], CommandLine.arguments.count > 4 ? CommandLine.arguments[4] : "")
    v.layer?.backgroundColor = NSColor(calibratedWhite: 0.16, alpha: 1).cgColor
    if let rep = v.bitmapImageRepForCachingDisplay(in: v.bounds) {
        v.cacheDisplay(in: v.bounds, to: rep)
        try? rep.representation(using: .png, properties: [:])?.write(to: URL(fileURLWithPath: CommandLine.arguments[2]))
    }
    exit(0)
}
if CommandLine.arguments.count >= 3, CommandLine.arguments[1] == "--test-toast" {
    // Manual check of the banner: Netleash.app/Contents/MacOS/Netleash --test-toast "Title" "Text"
    let a = NSApplication.shared
    a.setActivationPolicy(.accessory)
    Toast.show(CommandLine.arguments[2], CommandLine.arguments.count > 3 ? CommandLine.arguments[3] : "", seconds: 4)
    DispatchQueue.main.asyncAfter(deadline: .now() + 5) { exit(0) }
    a.run()
}

let app = NSApplication.shared
let delegate = App()
app.delegate = delegate
app.setActivationPolicy(.accessory)
app.run()
