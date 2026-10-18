/* Demo console: network simulation, control bar, event log, latency probe.
 * Runs before /widget.js so the simulations are in place when the widget boots. */
(function () {
  var apiUrl = window.__DEMO__.apiUrl;
  var projectKey = window.__DEMO__.projectKey;
  var params = new URLSearchParams(location.search);
  var log = [];
  var state = { latency: [], sentAt: 0 };

  // ---- network simulations -------------------------------------------------
  if (params.get("offline") === "1") {
    var blocked = function () {
      return Promise.reject(new TypeError("simulated offline"));
    };
    window.fetch = blocked;
    window.WebSocket = function () {
      throw new Error("simulated offline");
    };
  } else if (params.get("ws") === "0") {
    // Force the SSE -> long-poll fallback chain by removing WebSocket.
    window.WebSocket = function () {
      throw new Error("websocket disabled by the demo console");
    };
  }

  // ---- widget configuration ------------------------------------------------
  var eve = params.get("eve") === "1";
  var config = {
    projectKey: projectKey,
    apiUrl: apiUrl,
    theme: params.get("theme") === "dark" ? "dark" : params.get("theme") === "light" ? "light" : "auto",
    locale: params.get("locale") === "en" ? "en" : "fa",
    position: params.get("position") === "bottom-right" ? "bottom-right" : "bottom-left",
    font: params.get("font") === "none" ? "none" : params.get("font") === "inherit" ? "inherit" : "vazirmatn",
    visitor: {
      name: "کاربر دمو",
      mobile: "09121234567",
      metadata: { source: "demo" }
    }
  };
  if (eve) {
    config.visitor.metadata = { eve_user_id: "u_demo_1", eve_reseller_id: "reseller_demo" };
    config.visitor.mobile = "09120000000";
  }
  window.EveChat = config;
  window.ec = window.ec || function () { (ec.q = ec.q || []).push(arguments); };

  // ---- helpers -------------------------------------------------------------
  function record(kind, text, data) {
    var line = {
      at: new Date().toLocaleTimeString("fa-IR"),
      kind: kind,
      text: text,
      data: data
    };
    log.unshift(line);
    if (log.length > 200) log.pop();
    render();
  }

  function render() {
    var box = document.getElementById("demo-log");
    if (box) {
      box.innerHTML = log
        .map(function (l) {
          return '<div class="log-line"><span class="log-time">' + l.at + "</span> " + '<b class="log-kind">' + l.kind + "</b> " + escapeHtml(l.text);
        })
        .join("");
    }
    var latency = document.getElementById("demo-latency");
    if (latency) {
      if (!state.latency.length) {
        latency.textContent = "—";
      } else {
        var last = state.latency[state.latency.length - 1];
        var avg = state.latency.reduce(function (a, b) { return a + b; }, 0) / state.latency.length;
        latency.textContent = last + " ms (میانگین " + Math.round(avg) + " ms · " + state.latency.length + " نمونه)";
      }
    }
    var snap = document.getElementById("demo-state");
    if (snap) {
      var s = window.EveChat && window.EveChat.getState ? window.EveChat.getState() : null;
      snap.textContent = s
        ? JSON.stringify({ ready: s.ready, open: s.open, connection: s.connection, messages: s.messages.length, unread: s.unread, error: s.error }, null, 2)
        : "(بدون وضعیت)";
    }
  }

  function escapeHtml(v) {
    return String(v).replace(/[&<>"]/g, function (ch) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[ch];
    });
  }

  function reloadWith(key, value) {
    var p = new URLSearchParams(location.search);
    if (value === null || value === undefined || value === "") p.delete(key);
    else p.set(key, value);
    location.search = p.toString();
  }

  // ---- control bar ---------------------------------------------------------
  document.addEventListener("click", function (ev) {
    var el = ev.target.closest("[data-action]");
    if (!el) return;
    var action = el.getAttribute("data-action");
    var api = window.EveChat || {};
    switch (action) {
      case "open": api.open && api.open(); record("ctl", "ec('open')"); break;
      case "close": api.close && api.close(); record("ctl", "ec('close')"); break;
      case "toggle": api.toggle && api.toggle(); record("ctl", "ec('toggle')"); break;
      case "send": {
        // Measure the ack round trip: the sender does not receive its own
        // message.received frame, so poll the widget state until the message
        // flips to "sent" (the server acknowledged it).
        var startedAt = performance.now();
        api.send && api.send("پیام دمو " + new Date().toLocaleTimeString("fa-IR"));
        record("ctl", "پیام آزمایشی فرستاده شد");
        waitForAck(startedAt, 0);
        break;
      }
      case "identify":
        api.identify && api.identify({ name: "مهمان " + Math.floor(Math.random() * 999), mobile: "09120000001" });
        record("ctl", "ec('identify')");
        break;
      case "variable":
        api.setVariable && api.setVariable("plan", "pro");
        record("ctl", "ec('setVariable', plan=pro)");
        break;
      case "csat": record("ctl", "prompt csat در پنل نمایش داده می‌شود"); break;
      case "offline": reloadWith("offline", params.get("offline") === "1" ? "" : "1"); break;
      case "ws": reloadWith("ws", params.get("ws") === "0" ? "" : "0"); break;
      case "theme": reloadWith("theme", params.get("theme") === "dark" ? "light" : "dark"); break;
      case "locale": reloadWith("locale", params.get("locale") === "en" ? "fa" : "en"); break;
      case "position": reloadWith("position", params.get("position") === "bottom-right" ? "bottom-left" : "bottom-right"); break;
      case "font": reloadWith("font", params.get("font") === "none" ? "" : "none"); break;
      case "eve": reloadWith("eve", params.get("eve") === "1" ? "" : "1"); break;
      case "interference": {
        var style = document.getElementById("hostile-css");
        if (style) style.disabled = !style.disabled;
        record("shop", "تداخل CSS " + (style && !style.disabled ? "روشن" : "خاموش"));
        break;
      }
      case "clear":
        log.length = 0;
        state.latency.length = 0;
        render();
        break;
      case "reload": location.reload(); break;
      default: break;
    }
  });

  function waitForAck(startedAt, attempt) {
    var api = window.EveChat || {};
    var s = api.getState ? api.getState() : null;
    var mine = s
      ? s.messages.filter(function (m) { return m.sender_type === "visitor"; }).slice(-1)[0]
      : null;
    if (mine && mine.status === "sent") {
      var ms = Math.round(performance.now() - startedAt);
      state.latency.push(ms);
      record("latency", ms + " ms");
      return;
    }
    if (attempt < 200) {
      setTimeout(function () { waitForAck(startedAt, attempt + 1); }, 40);
    }
  }

  // ---- widget events -------------------------------------------------------
  function wire(attempt) {
    var api = window.EveChat;
    if (!api || !api.on) {
      if (attempt < 60) setTimeout(function () { wire(attempt + 1); }, 250);
      return;
    }
    ["ready", "open", "close", "connection", "message", "csat"].forEach(function (name) {
      api.on(name, function (data) {
        if (name === "message" && state.sentAt) {
          var ms = Math.round(performance.now() - state.sentAt);
          state.latency.push(ms);
          state.sentAt = 0;
          record("latency", ms + " ms", data);
        } else {
          record(name, typeof data === "string" ? data : JSON.stringify(data || {}).slice(0, 160));
        }
        render();
      });
    });
    record("ready", "اتصال به رویدادهای ویجت برقرار شد");
  }

  // ---- boot ----------------------------------------------------------------
  var script = document.createElement("script");
  script.async = true;
  script.src = apiUrl + "/widget.js";
  document.addEventListener("DOMContentLoaded", function () {
    document.body.appendChild(script);
    wire(0);
  });
})();