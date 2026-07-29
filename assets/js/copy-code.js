;(function () {
  var status = document.querySelector("[data-copy-status]")
  var template = document.querySelector("[data-copy-code-template]")
  if (!template) return

  Array.prototype.forEach.call(
    document.querySelectorAll(".codeblock[data-copyable]"),
    function (block) {
      if (block.querySelector(".copy-code")) return

      var code = block.querySelector("code")
      if (!code) return

      var language = (block.getAttribute("data-lang") || "").trim().toLowerCase()
      var button = template.content.firstElementChild.cloneNode(true)
      button.setAttribute(
        "aria-label",
        "Copy " + (language ? language + " " : "") + "code to clipboard"
      )
      block.insertBefore(button, block.firstChild)
    }
  )

  function flash(button) {
    if (button._label == null) button._label = button.getAttribute("aria-label")
    button.classList.add("copied")
    button.setAttribute("aria-label", "Copied to clipboard")
    if (status) status.textContent = "Copied to clipboard"

    window.clearTimeout(button._copyTimer)
    button._copyTimer = window.setTimeout(function () {
      button.classList.remove("copied")
      button.setAttribute("aria-label", button._label)
      if (status) status.textContent = ""
    }, 2000)
  }

  function fallbackCopy(text, button) {
    var textarea = document.createElement("textarea")
    textarea.value = text
    textarea.setAttribute("readonly", "")
    textarea.style.position = "absolute"
    textarea.style.left = "-9999px"
    document.body.appendChild(textarea)
    textarea.select()

    var copied = false
    try {
      copied = document.execCommand("copy")
    } catch (e) {}
    document.body.removeChild(textarea)
    button.focus()
    if (copied) flash(button)
  }

  function copy(text, button) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(
        function () {
          flash(button)
        },
        function () {
          fallbackCopy(text, button)
        }
      )
    } else {
      fallbackCopy(text, button)
    }
  }

  document.addEventListener("click", function (event) {
    var button = event.target.closest && event.target.closest(".copy-code")
    if (!button) return

    var block = button.closest(".codeblock")
    var code = block && block.querySelector("code")
    if (code) copy(code.innerText, button)
  })
})()
