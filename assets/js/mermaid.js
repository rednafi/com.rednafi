;(function () {
  var controller = document.currentScript
  var runtimeSource = controller && controller.getAttribute("data-mermaid-runtime")
  var diagrams = document.querySelectorAll(".mermaid")
  if (!runtimeSource || !diagrams.length) return

  diagrams.forEach(function (element) {
    element.setAttribute("data-source", element.textContent)
  })

  var renderChain = Promise.resolve()
  function renderMermaid() {
    renderChain = renderChain
      .catch(function () {})
      .then(function () {
        mermaid.initialize({
          startOnLoad: false,
          theme:
            document.documentElement.getAttribute("data-theme") === "dark"
              ? "dark"
              : "default",
          flowchart: { useMaxWidth: false },
          sequence: { useMaxWidth: false },
          state: { useMaxWidth: false },
          class: { useMaxWidth: false },
          er: { useMaxWidth: false }
        })

        diagrams.forEach(function (element) {
          var source = element.getAttribute("data-source")
          if (!source) return
          element.removeAttribute("data-processed")
          element.innerHTML = source
        })
        return mermaid.run()
      })
    return renderChain
  }

  var loading = false
  var observer
  function loadRuntime() {
    if (loading) return
    loading = true
    var script = document.createElement("script")
    script.src = runtimeSource
    script.async = true
    script.addEventListener("load", function () {
      if (typeof mermaid === "undefined") {
        loading = false
        script.remove()
        return
      }
      if (observer) observer.disconnect()
      window.__mermaidRerender = renderMermaid
      renderMermaid().catch(function () {})
    })
    script.addEventListener("error", function () {
      loading = false
      script.remove()
    })
    document.head.appendChild(script)
  }

  if ("IntersectionObserver" in window) {
    observer = new IntersectionObserver(
      function (entries) {
        if (
          !entries.some(function (entry) {
            return entry.isIntersecting
          })
        )
          return
        loadRuntime()
      },
      { rootMargin: "0px" }
    )
    diagrams.forEach(function (element) {
      observer.observe(element)
    })
  } else {
    loadRuntime()
  }
})()
