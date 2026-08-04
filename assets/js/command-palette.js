;(function () {
  var dialog = document.querySelector("[data-command-palette]")
  var panel = dialog && dialog.querySelector(".command-palette__dialog")
  var trigger = document.querySelector("[data-command-open]")
  var input = dialog && dialog.querySelector("[data-command-input]")
  var results = dialog && dialog.querySelector("[data-command-results]")
  var label = dialog && dialog.querySelector("[data-command-group-label]")
  var empty = dialog && dialog.querySelector("[data-command-empty]")
  var status = dialog && dialog.querySelector("[data-command-status]")
  var template = dialog && dialog.querySelector("[data-command-quick-links]")
  if (
    !dialog ||
    !panel ||
    !trigger ||
    !input ||
    !results ||
    !label ||
    !empty ||
    !status ||
    !template
  )
    return

  var quickLinks = Array.prototype.map.call(
    template.content.querySelectorAll("[data-title]"),
    function (link) {
      return {
        title: link.dataset.title || "",
        description: link.dataset.description || "",
        keywords: link.dataset.keywords || "",
        shortcut: link.dataset.commandShortcut || "",
        action: link.dataset.commandAction || "",
        url: link.getAttribute("href") || "/"
      }
    }
  )
  var pagefind
  var timer
  var sequence = 0
  var active = -1
  var lastFocus
  var chord = ""
  var chordTimer
  var shortcuts = {}

  quickLinks.forEach(function (link) {
    var keys = link.shortcut.split(/\s+/)
    if (keys.length !== 2) return
    shortcuts[keys[0]] = shortcuts[keys[0]] || {}
    shortcuts[keys[0]][keys[1]] = link.action || link.url
  })

  function normalized(value) {
    var text = String(value || "").toLowerCase()
    return text.normalize ? text.normalize("NFD").replace(/[\u0300-\u036f]/g, "") : text
  }

  function normalizedURL(value) {
    try {
      var url = new URL(value, location.origin)
      return url.pathname.replace(/index\.html$/, "") + url.hash
    } catch (_) {
      return value
    }
  }

  function sectionName(value) {
    var section = normalizedURL(value).split("/").filter(Boolean)[0] || "Page"
    return (
      {
        go: "Go",
        javascript: "JavaScript",
        misc: "Misc",
        python: "Python",
        shards: "Note",
        system: "Systems",
        typescript: "TypeScript",
        zephyr: "Essay"
      }[section] || "Page"
    )
  }

  function resultNode(item, source, index) {
    var link = document.createElement(item.action ? "button" : "a")
    if (item.action) {
      link.type = "button"
      link.dataset.commandAction = item.action
      link.addEventListener("click", function () {
        document.dispatchEvent(new CustomEvent("site:toggle-theme"))
        closePalette()
      })
    } else {
      link.href = item.url
    }
    link.id = "command-palette-result-" + index
    link.dataset.commandResult = ""
    link.dataset.commandSource = source
    link.setAttribute("role", "option")
    link.setAttribute("aria-selected", "false")
    link.tabIndex = -1

    var copy = document.createElement("span")
    copy.className = "command-palette__result-copy"
    var title = document.createElement("strong")
    title.textContent = (item.meta && item.meta.title) || item.title || "Untitled"
    var excerpt = document.createElement("small")
    excerpt.textContent = item.plain_excerpt || item.description || "Open this result"
    copy.append(title, excerpt)

    var kind = document.createElement("span")
    kind.className = "command-palette__result-meta"
    if (source === "quick-link" && item.shortcut) {
      kind.classList.add("command-palette__shortcut")
      item.shortcut.split(/\s+/).forEach(function (key) {
        var keycap = document.createElement("kbd")
        keycap.textContent = key.toUpperCase()
        kind.appendChild(keycap)
      })
    } else {
      kind.textContent = sectionName(item.url)
    }

    link.append(copy, kind)
    return link
  }

  function options() {
    return Array.prototype.slice.call(results.querySelectorAll("[data-command-result]"))
  }

  function setActive(index, scroll) {
    var items = options()
    if (!items.length) {
      active = -1
      input.removeAttribute("aria-activedescendant")
      return
    }
    active = (index + items.length) % items.length
    items.forEach(function (item, position) {
      var selected = position === active
      item.classList.toggle("is-active", selected)
      item.setAttribute("aria-selected", String(selected))
    })
    input.setAttribute("aria-activedescendant", items[active].id)
    if (scroll) items[active].scrollIntoView({ block: "nearest" })
  }

  function render(items, heading, loading) {
    results.replaceChildren()
    items.slice(0, 8).forEach(function (item, index) {
      results.appendChild(resultNode(item.data, item.source, index))
    })
    label.textContent = heading
    label.hidden = !items.length && !loading
    empty.hidden = loading || !!items.length
    results.setAttribute("aria-busy", String(!!loading))
    setActive(items.length ? 0 : -1, false)
  }

  function showQuickLinks() {
    render(
      quickLinks.map(function (link) {
        return { data: link, source: "quick-link" }
      }),
      "Navigate",
      false
    )
    status.textContent = quickLinks.length + " navigation links"
  }

  function search(query) {
    clearTimeout(timer)
    var request = ++sequence
    if (!query) {
      showQuickLinks()
      return
    }

    var words = normalized(query).split(/\s+/).filter(Boolean)
    var matches = quickLinks.filter(function (link) {
      var haystack = normalized([link.title, link.description, link.keywords].join(" "))
      return words.every(function (word) {
        return haystack.indexOf(word) !== -1
      })
    })
    render(
      matches.map(function (link) {
        return { data: link, source: "quick-link" }
      }),
      matches.length ? "Top matches" : "Searching…",
      true
    )
    status.textContent = "Searching for " + query

    timer = setTimeout(function () {
      pagefind = pagefind || import("/pagefind/pagefind.js")
      pagefind
        .then(function (api) {
          return api.search(query)
        })
        .then(function (found) {
          return Promise.all(
            found.results.slice(0, 8).map(function (result) {
              return result.data()
            })
          )
        })
        .then(function (found) {
          if (request !== sequence) return
          var seen = new Set(
            matches.map(function (link) {
              return normalizedURL(link.url)
            })
          )
          var combined = matches.map(function (link) {
            return { data: link, source: "quick-link" }
          })
          found.forEach(function (item) {
            var url = normalizedURL(item.url)
            if (!seen.has(url)) {
              seen.add(url)
              combined.push({ data: item, source: "pagefind" })
            }
          })
          render(combined, "Search results", false)
          status.textContent = Math.min(combined.length, 8) + " results for " + query
        })
        .catch(function () {
          if (request !== sequence) return
          pagefind = null
          render(
            matches.map(function (link) {
              return { data: link, source: "quick-link" }
            }),
            matches.length ? "Top matches" : "Search unavailable",
            false
          )
          status.textContent = "The search index could not be loaded"
        })
    }, 90)
  }

  function openPalette(event) {
    if (event) event.preventDefault()
    if (dialog.open) return
    lastFocus = document.activeElement
    var nav = document.querySelector("[data-nav-toggle][aria-expanded='true']")
    if (nav) nav.click()
    input.value = ""
    showQuickLinks()
    dialog.showModal()
    trigger.setAttribute("aria-expanded", "true")
    input.focus()
  }

  function closePalette() {
    if (dialog.open) dialog.close()
  }

  trigger.addEventListener("click", openPalette)
  dialog.querySelector("[data-command-close]").addEventListener("click", closePalette)
  dialog.addEventListener("click", function (event) {
    if (event.target === dialog) closePalette()
  })
  dialog.addEventListener("close", function () {
    clearTimeout(timer)
    sequence += 1
    input.value = ""
    trigger.setAttribute("aria-expanded", "false")
    if (lastFocus && lastFocus.focus) lastFocus.focus()
  })
  input.addEventListener("input", function () {
    search(input.value.trim())
  })
  input.addEventListener("keydown", function (event) {
    if (event.key === "Escape") {
      event.preventDefault()
      closePalette()
    } else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault()
      setActive(active + (event.key === "ArrowDown" ? 1 : -1), true)
    } else if (event.key === "Enter") {
      var items = options()
      if (items[active]) {
        event.preventDefault()
        items[active].click()
      }
    }
  })
  results.addEventListener("mousemove", function (event) {
    var item = event.target.closest("[data-command-result]")
    if (item) setActive(options().indexOf(item), false)
  })
  function resetChord() {
    chord = ""
    clearTimeout(chordTimer)
  }
  document.addEventListener("keydown", function (event) {
    if (event.defaultPrevented || event.isComposing || event.repeat) return
    var interactive =
      event.target.closest &&
      event.target.closest("a, button, input, textarea, select, [contenteditable]")
    if (interactive || event.metaKey || event.ctrlKey || event.altKey) {
      resetChord()
      return
    }
    var key = event.key.toLowerCase()
    if (key === "/" || key === "?") {
      resetChord()
      openPalette(event)
      return
    }
    if (chord && shortcuts[chord] && shortcuts[chord][key]) {
      event.preventDefault()
      var command = shortcuts[chord][key]
      resetChord()
      if (command === "theme") document.dispatchEvent(new CustomEvent("site:toggle-theme"))
      else location.assign(command)
      return
    }
    resetChord()
    if (shortcuts[key]) {
      chord = key
      chordTimer = setTimeout(resetChord, 1200)
    }
  })
})()
