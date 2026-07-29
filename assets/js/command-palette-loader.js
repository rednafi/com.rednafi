;(function () {
  var script = document.currentScript
  var source = script && script.getAttribute("data-command-module")
  var trigger = document.querySelector("[data-command-open]")
  var palette = document.querySelector("[data-command-palette]")
  var input = document.querySelector("[data-command-input]")
  if (!source || !trigger || !palette || !input) return

  var loading
  var pending = false
  var ready = false

  function usesCommandKey() {
    var platform =
      (navigator.userAgentData && navigator.userAgentData.platform) ||
      navigator.platform ||
      ""
    return /mac|iphone|ipad|ipod/i.test(platform)
  }

  var modifier = document.querySelector("[data-command-modifier]")
  if (modifier) modifier.textContent = usesCommandKey() ? "⌘" : "Ctrl"

  function loadController() {
    if (!loading) {
      loading = import(source).then(function () {
        ready = true
      })
    }
    return loading
  }

  function toggle(lastFocus) {
    document.dispatchEvent(
      new CustomEvent("command-palette:toggle", {
        detail: { lastFocus: lastFocus, query: input.value }
      })
    )
  }

  function requestToggle(event) {
    if (event) event.preventDefault()
    if (pending) return
    var lastFocus = document.activeElement
    if (ready) {
      toggle(lastFocus)
      return
    }

    pending = true
    palette.hidden = false
    trigger.setAttribute("aria-expanded", "true")
    input.focus({ preventScroll: true })
    loadController().then(
      function () {
        pending = false
        toggle(lastFocus)
      },
      function () {
        pending = false
        loading = null
        palette.hidden = true
        trigger.setAttribute("aria-expanded", "false")
        if (lastFocus && typeof lastFocus.focus === "function") lastFocus.focus()
      }
    )
  }

  trigger.addEventListener("click", requestToggle)
  document.addEventListener("keydown", function (event) {
    var commandShortcut =
      event.key.toLowerCase() === "k" &&
      (event.metaKey || event.ctrlKey) &&
      !event.altKey &&
      !event.shiftKey &&
      !event.repeat
    if (commandShortcut) requestToggle(event)
  })
})()
