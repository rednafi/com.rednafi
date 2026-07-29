;(function () {
  function initThemeSwitcher() {
    var root = document.documentElement
    var media = window.matchMedia("(prefers-color-scheme: dark)")
    var themeMeta = document.querySelector('meta[name="theme-color"][data-theme-color]')
    var buttons = Array.prototype.slice.call(document.querySelectorAll("[data-theme-set]"))

    function storedPreference() {
      try {
        return localStorage.getItem("theme") || "system"
      } catch (e) {
        return "system"
      }
    }

    function writePreference(preference) {
      try {
        localStorage.setItem("theme", preference)
      } catch (e) {}
    }

    function resolvedTheme(preference) {
      return preference === "dark" || (preference === "system" && media.matches)
        ? "dark"
        : "light"
    }

    function syncButtons(preference) {
      buttons.forEach(function (button) {
        var active = button.getAttribute("data-theme-set") === preference
        button.setAttribute("aria-checked", String(active))
        button.tabIndex = active ? 0 : -1
      })
    }

    function applyTheme(preference, persist) {
      var theme = resolvedTheme(preference)
      var changed = root.getAttribute("data-theme") !== theme
      if (changed) root.setAttribute("data-theme", theme)
      if (root.getAttribute("data-theme-preference") !== preference) {
        root.setAttribute("data-theme-preference", preference)
      }
      if (persist) writePreference(preference)
      if (
        themeMeta &&
        themeMeta.getAttribute("content") !== (theme === "dark" ? "#0a0a0a" : "#fafafa")
      ) {
        themeMeta.setAttribute("content", theme === "dark" ? "#0a0a0a" : "#fafafa")
      }
      syncButtons(preference)
      if (changed && window.__mermaidRerender) window.__mermaidRerender()
    }

    buttons.forEach(function (button) {
      button.addEventListener("click", function () {
        applyTheme(button.getAttribute("data-theme-set"), true)
      })

      button.addEventListener("keydown", function (event) {
        var keys = ["ArrowLeft", "ArrowUp", "ArrowRight", "ArrowDown", "Home", "End"]
        if (keys.indexOf(event.key) === -1) return

        var index = buttons.indexOf(button)
        if (index === -1) return
        event.preventDefault()

        if (event.key === "Home") index = 0
        else if (event.key === "End") index = buttons.length - 1
        else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
          index = (index - 1 + buttons.length) % buttons.length
        } else {
          index = (index + 1) % buttons.length
        }

        buttons[index].focus()
        applyTheme(buttons[index].getAttribute("data-theme-set"), true)
      })
    })

    if (media.addEventListener) {
      media.addEventListener("change", function () {
        if (
          (root.getAttribute("data-theme-preference") || storedPreference()) === "system"
        ) {
          applyTheme("system", false)
        }
      })
    }

    applyTheme(storedPreference(), false)
  }

  function initNavigation() {
    var wrapper = document.querySelector("[data-nav]")
    var toggle = document.querySelector("[data-nav-toggle]")
    var menu = document.querySelector("[data-nav-menu]")
    if (!wrapper || !toggle || !menu) return

    var header = wrapper.closest(".site-header") || wrapper
    var hoverable = window.matchMedia("(hover: hover) and (pointer: fine)").matches
    var closeTimer
    var hideTimer

    function isOpen() {
      return toggle.getAttribute("aria-expanded") === "true"
    }

    function open() {
      window.clearTimeout(closeTimer)
      window.clearTimeout(hideTimer)
      if (isOpen()) return

      menu.hidden = false
      toggle.setAttribute("aria-expanded", "true")
      toggle.setAttribute("aria-label", "Close menu")
      requestAnimationFrame(function () {
        if (isOpen()) menu.classList.add("is-open")
      })
      document.addEventListener("keydown", onKey)
      document.addEventListener("click", onOutside, true)
    }

    function close() {
      window.clearTimeout(closeTimer)
      if (!isOpen()) return

      menu.classList.remove("is-open")
      toggle.setAttribute("aria-expanded", "false")
      toggle.setAttribute("aria-label", "Open menu")
      document.removeEventListener("keydown", onKey)
      document.removeEventListener("click", onOutside, true)

      var hide = function (event) {
        if (event && (event.target !== menu || event.propertyName !== "opacity")) return
        window.clearTimeout(hideTimer)
        menu.removeEventListener("transitionend", hide)
        if (!isOpen()) menu.hidden = true
      }
      menu.addEventListener("transitionend", hide)
      hideTimer = window.setTimeout(hide, 250)
    }

    function toggleMenu() {
      isOpen() ? close() : open()
    }

    function onKey(event) {
      if (event.key !== "Escape") return
      close()
      toggle.focus()
    }

    function onOutside(event) {
      if (!wrapper.contains(event.target)) close()
    }

    toggle.addEventListener("click", toggleMenu)

    if (hoverable) {
      wrapper.addEventListener("mouseenter", open)
      header.addEventListener("mouseenter", function () {
        window.clearTimeout(closeTimer)
      })
      header.addEventListener("mouseleave", function () {
        window.clearTimeout(closeTimer)
        closeTimer = window.setTimeout(function () {
          if (!menu.contains(document.activeElement)) close()
        }, 160)
      })
      header.addEventListener("focusout", function (event) {
        if (!header.contains(event.relatedTarget)) close()
      })
    }
  }

  function initBackToTop() {
    var button = document.querySelector(".back-to-top")
    if (!button) return
    var visibleState

    function setVisible(visible) {
      if (visible === visibleState) return
      visibleState = visible
      button.classList.toggle("visible", visible)
      button.setAttribute("aria-hidden", visible ? "false" : "true")
      button.tabIndex = visible ? 0 : -1
    }

    window.addEventListener(
      "scroll",
      function () {
        setVisible(window.scrollY > 300)
      },
      { passive: true }
    )
    button.addEventListener("click", function (event) {
      event.preventDefault()
      window.scrollTo({
        top: 0,
        behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches
          ? "auto"
          : "smooth"
      })
    })
    setVisible(window.scrollY > 300)
  }

  initThemeSwitcher()
  initNavigation()
  initBackToTop()
})()
