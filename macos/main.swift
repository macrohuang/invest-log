import AppKit
import WebKit

@available(macOS 11.0, *)
private func clearWebViewWebsiteData() {
  let dataStore = WKWebsiteDataStore.default()
  let allTypes = WKWebsiteDataStore.allWebsiteDataTypes()
  dataStore.fetchDataRecords(ofTypes: allTypes) { records in
    dataStore.removeData(ofTypes: allTypes, for: records) {}
  }
}

class AppDelegate: NSObject, NSApplicationDelegate {
  private var window: NSWindow!
  private var webView: WKWebView!
  private var backendProcess: Process?

  private let host = "127.0.0.1"
  private let port = 8000
  private let maxAttempts = 80

  func applicationDidFinishLaunching(_ notification: Notification) {
    if #available(macOS 11.0, *) {
      clearWebViewWebsiteData()
    }
    setupMenu()
    setupWindow()
    loadLoadingScreen()
    startBackend()
    waitForServer(attempt: 0)
  }

  func applicationWillTerminate(_ notification: Notification) {
    backendProcess?.terminate()
  }

  func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
    return true
  }

  private func setupWindow() {
    let config = WKWebViewConfiguration()
    webView = WKWebView(frame: .zero, configuration: config)

    window = NSWindow(
      contentRect: NSRect(x: 0, y: 0, width: 1200, height: 800),
      styleMask: [.titled, .closable, .miniaturizable, .resizable],
      backing: .buffered,
      defer: false
    )
    window.center()
    window.title = "Invest Log"
    window.contentView = webView
    window.makeKeyAndOrderFront(nil)
  }

  private func loadLoadingScreen() {
    if let url = Bundle.main.url(forResource: "loading", withExtension: "html") {
      webView.loadFileURL(url, allowingReadAccessTo: url.deletingLastPathComponent())
    } else {
      webView.loadHTMLString("<!doctype html><html><body><p>Loading…</p></body></html>", baseURL: nil)
    }
  }

  private func startBackend() {
    guard let resourcePath = Bundle.main.resourcePath else {
      showFatalError("Missing app resources.")
      return
    }

    let backendURL = URL(fileURLWithPath: resourcePath).appendingPathComponent("invest-log-backend")
    let webDirURL = URL(fileURLWithPath: resourcePath).appendingPathComponent("static")

    let process = Process()
    process.executableURL = backendURL
    process.arguments = [
      "--host", host,
      "--port", "\(port)",
      "--web-dir", webDirURL.path
    ]
    process.currentDirectoryURL = URL(fileURLWithPath: resourcePath)
    var env = ProcessInfo.processInfo.environment
    env["INVEST_LOG_PARENT_WATCH"] = "1"
    process.environment = env

    do {
      try process.run()
      backendProcess = process
    } catch {
      showFatalError("Unable to start backend. \(error.localizedDescription)")
    }
  }

  private func waitForServer(attempt: Int) {
    let url = URL(string: "http://\(host):\(port)/api/health")!
    var request = URLRequest(url: url)
    request.timeoutInterval = 1.0

    URLSession.shared.dataTask(with: request) { [weak self] _, response, _ in
      guard let self = self else { return }
      if let http = response as? HTTPURLResponse, http.statusCode == 200 {
        DispatchQueue.main.async { self.loadApp() }
        return
      }
      if attempt < self.maxAttempts {
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.25) {
          self.waitForServer(attempt: attempt + 1)
        }
      } else {
        DispatchQueue.main.async { self.loadApp() }
      }
    }.resume()
  }

  private func loadApp() {
    let url = URL(string: "http://\(host):\(port)/")!
    webView.load(URLRequest(url: url))
  }

  private func setupMenu() {
    let mainMenu = NSMenu()

    // Application menu (first item is always the app menu on macOS)
    let appMenuItem = NSMenuItem()
    mainMenu.addItem(appMenuItem)
    let appMenu = NSMenu()
    appMenuItem.submenu = appMenu
    appMenu.addItem(withTitle: "Quit Invest Log", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")

    // Edit menu — routes standard edit commands through the responder chain to WKWebView
    let editMenuItem = NSMenuItem()
    mainMenu.addItem(editMenuItem)
    let editMenu = NSMenu(title: "Edit")
    editMenuItem.submenu = editMenu
    editMenu.addItem(withTitle: "Undo", action: #selector(UndoManager.undo), keyEquivalent: "z")
    editMenu.addItem(withTitle: "Redo", action: #selector(UndoManager.redo), keyEquivalent: "Z")
    editMenu.addItem(NSMenuItem.separator())
    editMenu.addItem(withTitle: "Cut", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
    editMenu.addItem(withTitle: "Copy", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
    editMenu.addItem(withTitle: "Paste", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
    editMenu.addItem(NSMenuItem.separator())
    editMenu.addItem(withTitle: "Select All", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")

    NSApp.mainMenu = mainMenu
  }

  private func showFatalError(_ message: String) {
    DispatchQueue.main.async {
      let alert = NSAlert()
      alert.messageText = "Invest Log"
      alert.informativeText = message
      alert.alertStyle = .critical
      alert.runModal()
      NSApp.terminate(nil)
    }
  }
}

let app = NSApplication.shared
app.setActivationPolicy(.regular)
let delegate = AppDelegate()
app.delegate = delegate
app.activate(ignoringOtherApps: true)
app.run()
