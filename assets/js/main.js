// Menu scroll position persistence (deferred to avoid forced reflow during load)
;(function () {
  var menu = document.getElementById("menu")
  if (menu) {
    // Defer scroll restoration to after paint to avoid blocking LCP
    requestAnimationFrame(function () {
      var savedPosition = localStorage.getItem("menu-scroll-position")
      if (savedPosition) {
        menu.scrollLeft = savedPosition
      }
    })
    var scrollTimeout
    menu.onscroll = function () {
      clearTimeout(scrollTimeout)
      scrollTimeout = setTimeout(function () {
        localStorage.setItem("menu-scroll-position", menu.scrollLeft)
      }, 150)
    }
  }
})()

// Smooth anchor scrolling with reduced-motion support
document.querySelectorAll('a[href^="#"]').forEach(function (anchor) {
  anchor.addEventListener("click", function (e) {
    e.preventDefault()
    var id = this.getAttribute("href").substr(1)
    var target = document.querySelector('[id="' + decodeURIComponent(id) + '"]')
    if (target) {
      if (!window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
        target.scrollIntoView({ behavior: "smooth" })
      } else {
        target.scrollIntoView()
      }
      if (id === "top") {
        history.replaceState(null, null, " ")
      } else {
        history.pushState(null, null, "#" + id)
      }
    }
  })
})

// Scroll-to-top button visibility
;(function () {
  var topButton = document.getElementById("top-link")
  if (topButton) {
    var ticking = false
    window.addEventListener("scroll", function () {
      if (!ticking) {
        window.requestAnimationFrame(function () {
          if (document.body.scrollTop > 800 || document.documentElement.scrollTop > 800) {
            topButton.style.visibility = "visible"
            topButton.style.opacity = "1"
          } else {
            topButton.style.visibility = "hidden"
            topButton.style.opacity = "0"
          }
          ticking = false
        })
        ticking = true
      }
    })
  }
})()

// Theme toggle
;(function () {
  var themeToggle = document.getElementById("theme-toggle")
  if (themeToggle) {
    themeToggle.addEventListener("click", function () {
      var html = document.documentElement
      var isDark = html.dataset.theme === "dark"
      var newTheme = isDark ? "light" : "dark"
      html.dataset.theme = newTheme
      document.body.classList.toggle("dark", newTheme === "dark")
      localStorage.setItem("pref-theme", newTheme)
    })
  }
})()

// Hamburger menu aria-expanded toggle
;(function () {
  var hamburgerToggle = document.getElementById("hamburger-toggle")
  var hamburgerLabel = document.querySelector(".hamburger-label")
  if (hamburgerToggle && hamburgerLabel) {
    hamburgerToggle.addEventListener("change", function () {
      hamburgerLabel.setAttribute("aria-expanded", this.checked ? "true" : "false")
    })
  }
})()

// Active TOC highlighting
;(function () {
  var toc = document.querySelector(".toc")
  if (!toc) return
  var headings = document.querySelectorAll(
    ".post-content h1[id], .post-content h2[id], .post-content h3[id], .post-content h4[id]"
  )
  if (!headings.length) return
  var tocLinks = toc.querySelectorAll("a")
  var observer = new IntersectionObserver(
    function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          tocLinks.forEach(function (link) {
            link.classList.remove("active")
          })
          var id = entry.target.getAttribute("id")
          var activeLink = toc.querySelector('a[href="#' + id + '"]')
          if (activeLink) activeLink.classList.add("active")
        }
      })
    },
    { rootMargin: "-80px 0px -66% 0px" }
  )
  headings.forEach(function (heading) {
    observer.observe(heading)
  })
})()

// Navigation drawer toggle
;(function () {
  var toggle = document.getElementById("drawer-toggle")
  var drawer = document.getElementById("site-drawer")
  var overlay = document.getElementById("drawer-overlay")

  if (!toggle || !drawer || !overlay) return

  function onEscape(e) {
    if (e.key === "Escape") closeDrawer()
  }

  function openDrawer() {
    drawer.classList.add("open")
    overlay.classList.add("active")
    document.body.classList.add("drawer-open")
    toggle.setAttribute("aria-expanded", "true")
    drawer.setAttribute("aria-hidden", "false")
    document.addEventListener("keydown", onEscape)
  }

  function closeDrawer() {
    if (!drawer.classList.contains("open")) return
    drawer.classList.remove("open")
    overlay.classList.remove("active")
    document.body.classList.remove("drawer-open")
    toggle.setAttribute("aria-expanded", "false")
    drawer.setAttribute("aria-hidden", "true")
    document.removeEventListener("keydown", onEscape)
  }

  function toggleDrawer() {
    if (drawer.classList.contains("open")) {
      closeDrawer()
    } else {
      openDrawer()
    }
  }

  toggle.addEventListener("click", toggleDrawer)
  overlay.addEventListener("click", closeDrawer)

  document.addEventListener("keydown", function (e) {
    if ((e.metaKey || e.ctrlKey) && e.key === "k") {
      e.preventDefault()
      toggleDrawer()
    }
  })
})()
