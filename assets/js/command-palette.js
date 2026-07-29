;(function () {
  var palette = document.querySelector("[data-command-palette]")
  var dialog = palette && palette.querySelector(".command-palette__dialog")
  var input = palette && palette.querySelector("[data-command-input]")
  var results = palette && palette.querySelector("[data-command-results]")
  var groupLabel = palette && palette.querySelector("[data-command-group-label]")
  var empty = palette && palette.querySelector("[data-command-empty]")
  var status = palette && palette.querySelector("[data-command-status]")
  var template = palette && palette.querySelector("[data-command-quick-links]")
  var pageShell = document.querySelector(".page")
  var trigger = document.querySelector("[data-command-open]")
  if (
    !palette ||
    !dialog ||
    !input ||
    !results ||
    !groupLabel ||
    !empty ||
    !status ||
    !template ||
    !trigger
  )
    return

  var quickLinks = Array.prototype.map.call(
    template.content.querySelectorAll("a"),
    function (link) {
      return {
        title: link.getAttribute("data-title") || "",
        description: link.getAttribute("data-description") || "",
        keywords: link.getAttribute("data-keywords") || "",
        url: link.getAttribute("href") || "/"
      }
    }
  )
  var pagefindPromise
  var searchTimer
  var searchSequence = 0
  var activeIndex = -1
  var lastFocus = null
  var openState = false

  function getPagefind() {
    if (!pagefindPromise) {
      pagefindPromise = import("/pagefind/pagefind.js").catch(function (error) {
        pagefindPromise = null
        throw error
      })
    }
    return pagefindPromise
  }

  function normalizedURL(url) {
    try {
      var parsed = new URL(url, window.location.origin)
      return parsed.pathname.replace(/index\.html$/, "") + parsed.hash
    } catch (e) {
      return url
    }
  }

  function matchesQuickLink(link, query) {
    var words = query.toLowerCase().split(/\s+/).filter(Boolean)
    var haystack = [link.title, link.description, link.keywords].join(" ").toLowerCase()
    return words.every(function (word) {
      return haystack.indexOf(word) !== -1
    })
  }

  function sectionName(url) {
    var special = {
      "/about/": "Page",
      "/appearances/": "Page",
      "/blogroll/": "Page",
      "/maxims/": "Page"
    }
    var path = normalizedURL(url).split("#")[0]
    if (special[path]) return special[path]
    var section = path.split("/").filter(Boolean)[0] || "Page"
    var names = {
      go: "Go",
      javascript: "JavaScript",
      misc: "Misc",
      python: "Python",
      shards: "Note",
      system: "Systems",
      typescript: "TypeScript",
      zephyr: "Essay"
    }
    return names[section] || "Writing"
  }

  function resultNode(result, source) {
    var link = document.createElement("a")
    link.href = result.url
    link.setAttribute("data-command-result", "")

    var icon = document.createElement("span")
    icon.className = "command-palette__result-icon"
    icon.setAttribute("aria-hidden", "true")
    icon.innerHTML =
      '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 2h9l4 4v16H6z"/><path d="M14 2v5h5"/><path d="M9 13h6"/><path d="M9 17h4"/></svg>'

    var copy = document.createElement("span")
    copy.className = "command-palette__result-copy"
    var title = document.createElement("strong")
    title.textContent = (result.meta && result.meta.title) || result.title || "Untitled"
    var excerpt = document.createElement("small")
    excerpt.textContent = result.plain_excerpt || result.description || "Open this result"
    copy.appendChild(title)
    copy.appendChild(excerpt)

    var kind = document.createElement("span")
    kind.className = "command-palette__result-kind"
    kind.textContent = source === "quick-link" ? "Page" : sectionName(result.url)

    var arrow = document.createElement("span")
    arrow.className = "command-palette__result-arrow"
    arrow.setAttribute("aria-hidden", "true")
    arrow.innerHTML =
      '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14"/><path d="m13 6 6 6-6 6"/></svg>'

    link.appendChild(icon)
    link.appendChild(copy)
    link.appendChild(kind)
    link.appendChild(arrow)
    return link
  }

  function resultLinks() {
    return Array.prototype.slice.call(results.querySelectorAll("[data-command-result]"))
  }

  function setActive(index, scroll) {
    var links = resultLinks()
    if (!links.length) {
      activeIndex = -1
      input.removeAttribute("aria-activedescendant")
      return
    }
    activeIndex = (index + links.length) % links.length
    links.forEach(function (link, i) {
      var active = i === activeIndex
      link.classList.toggle("is-active", active)
      link.setAttribute("aria-selected", String(active))
    })
    input.setAttribute("aria-activedescendant", links[activeIndex].id)
    if (scroll) links[activeIndex].scrollIntoView({ block: "nearest" })
  }

  function prepareResult(link, index, source) {
    link.id = "command-palette-result-" + index
    link.setAttribute("role", "option")
    link.setAttribute("aria-selected", "false")
    link.setAttribute("tabindex", "-1")
    link.setAttribute("data-command-source", source)
    link.addEventListener("mousemove", function () {
      setActive(resultLinks().indexOf(link), false)
    })
    return link
  }

  function render(nodes, label, loading) {
    results.replaceChildren()
    nodes.slice(0, 8).forEach(function (item, index) {
      var node = resultNode(item.data, item.source)
      results.appendChild(prepareResult(node, index, item.source))
    })

    var links = resultLinks()
    groupLabel.textContent = label
    groupLabel.hidden = !links.length && !loading
    empty.hidden = loading || !!links.length
    results.setAttribute("aria-busy", String(!!loading))
    setActive(links.length ? 0 : -1, false)
  }

  function renderQuickLinks() {
    render(
      quickLinks.map(function (link) {
        return { source: "quick-link", data: link }
      }),
      "Quick links",
      false
    )
    status.textContent = quickLinks.length + " quick links"
  }

  function beginSearch(query) {
    window.clearTimeout(searchTimer)
    searchSequence += 1
    var sequence = searchSequence
    if (!query) {
      renderQuickLinks()
      return
    }

    var matches = quickLinks.filter(function (link) {
      return matchesQuickLink(link, query)
    })
    render(
      matches.map(function (link) {
        return { source: "quick-link", data: link }
      }),
      matches.length ? "Top matches" : "Searching…",
      true
    )
    status.textContent = "Searching for " + query

    searchTimer = window.setTimeout(function () {
      getPagefind()
        .then(function (pagefind) {
          return pagefind.search(query)
        })
        .then(function (search) {
          if (sequence !== searchSequence) return []
          return Promise.all(
            search.results.slice(0, 8).map(function (result) {
              return result.data()
            })
          )
        })
        .then(function (pagefindResults) {
          if (sequence !== searchSequence) return
          var seen = new Set(
            matches.map(function (link) {
              return normalizedURL(link.url)
            })
          )
          var combined = matches.map(function (link) {
            return { source: "quick-link", data: link }
          })

          pagefindResults.forEach(function (result) {
            var url = normalizedURL(result.url)
            if (seen.has(url)) return
            seen.add(url)
            combined.push({ source: "pagefind", data: result })
          })

          render(combined, "Results", false)
          var count = Math.min(combined.length, 8)
          status.textContent =
            count + (count === 1 ? " result" : " results") + " for " + query
        })
        .catch(function () {
          if (sequence !== searchSequence) return
          render(
            matches.map(function (link) {
              return { source: "quick-link", data: link }
            }),
            matches.length ? "Top matches" : "Search unavailable",
            false
          )
          status.textContent = "The search index could not be loaded"
        })
    }, 90)
  }

  function openPalette(initialFocus, initialQuery) {
    if (openState) {
      input.focus()
      return
    }
    openState = true
    lastFocus = initialFocus || document.activeElement

    var navToggle = document.querySelector("[data-nav-toggle][aria-expanded='true']")
    if (navToggle) {
      lastFocus = navToggle
      navToggle.click()
    }

    palette.hidden = false
    if (pageShell) pageShell.setAttribute("inert", "")
    document.body.classList.add("command-palette-open")
    trigger.setAttribute("aria-expanded", "true")
    renderQuickLinks()
    input.value = initialQuery || ""
    if (input.value) beginSearch(input.value.trim())
    palette.setAttribute("data-command-ready", "")
    input.focus()
    document.addEventListener("keydown", handleDialogKeys)
    requestAnimationFrame(function () {
      if (!openState) return
      palette.classList.add("is-open")
    })
  }

  function closePalette() {
    if (!openState) return
    openState = false
    window.clearTimeout(searchTimer)
    searchSequence += 1
    palette.classList.remove("is-open")
    palette.hidden = true
    input.value = ""
    if (pageShell) pageShell.removeAttribute("inert")
    document.body.classList.remove("command-palette-open")
    trigger.setAttribute("aria-expanded", "false")
    document.removeEventListener("keydown", handleDialogKeys)
    if (lastFocus && typeof lastFocus.focus === "function") lastFocus.focus()
  }

  function handleDialogKeys(event) {
    if (!openState) return
    if (event.key === "Escape") {
      event.preventDefault()
      event.stopPropagation()
      closePalette()
      return
    }
    if (event.target === input && (event.key === "ArrowDown" || event.key === "ArrowUp")) {
      event.preventDefault()
      setActive(activeIndex + (event.key === "ArrowDown" ? 1 : -1), true)
      return
    }
    if (event.target === input && event.key === "Enter") {
      var links = resultLinks()
      if (activeIndex >= 0 && links[activeIndex]) {
        event.preventDefault()
        links[activeIndex].click()
      }
      return
    }
    if (event.key === "Tab") {
      var focusable = Array.prototype.slice.call(
        dialog.querySelectorAll(
          'input, button:not([disabled]), a[href]:not([tabindex="-1"])'
        )
      )
      if (!focusable.length) return
      var first = focusable[0]
      var last = focusable[focusable.length - 1]
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
  }

  trigger.setAttribute("aria-expanded", "false")
  document.addEventListener("command-palette:toggle", function (event) {
    openState
      ? closePalette()
      : openPalette(
          event.detail && event.detail.lastFocus,
          event.detail && event.detail.query
        )
  })

  Array.prototype.forEach.call(
    palette.querySelectorAll("[data-command-close]"),
    function (close) {
      close.addEventListener("click", closePalette)
    }
  )

  input.addEventListener("input", function () {
    beginSearch(input.value.trim())
  })
})()
