// Menu scroll position persistence
(function() {
    var menu = document.getElementById('menu');
    if (menu) {
        menu.scrollLeft = localStorage.getItem("menu-scroll-position");
        var scrollTimeout;
        menu.onscroll = function() {
            clearTimeout(scrollTimeout);
            scrollTimeout = setTimeout(function() {
                localStorage.setItem("menu-scroll-position", menu.scrollLeft);
            }, 150);
        };
    }
})();

// Smooth anchor scrolling with reduced-motion support
document.querySelectorAll('a[href^="#"]').forEach(function(anchor) {
    anchor.addEventListener("click", function(e) {
        e.preventDefault();
        var id = this.getAttribute("href").substr(1);
        var target = document.querySelector('[id="' + decodeURIComponent(id) + '"]');
        if (target) {
            if (!window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
                target.scrollIntoView({ behavior: "smooth" });
            } else {
                target.scrollIntoView();
            }
            if (id === "top") {
                history.replaceState(null, null, " ");
            } else {
                history.pushState(null, null, "#" + id);
            }
        }
    });
});

// Scroll-to-top button visibility
(function() {
    var topButton = document.getElementById("top-link");
    if (topButton) {
        var ticking = false;
        window.addEventListener('scroll', function() {
            if (!ticking) {
                window.requestAnimationFrame(function() {
                    if (document.body.scrollTop > 800 || document.documentElement.scrollTop > 800) {
                        topButton.style.visibility = "visible";
                        topButton.style.opacity = "1";
                    } else {
                        topButton.style.visibility = "hidden";
                        topButton.style.opacity = "0";
                    }
                    ticking = false;
                });
                ticking = true;
            }
        });
    }
})();

// Theme toggle
(function() {
    var themeToggle = document.getElementById("theme-toggle");
    if (themeToggle) {
        themeToggle.addEventListener("click", function() {
            if (document.body.className.includes("dark")) {
                document.body.classList.remove('dark');
                localStorage.setItem("pref-theme", 'light');
            } else {
                document.body.classList.add('dark');
                localStorage.setItem("pref-theme", 'dark');
            }
        });
    }
})();
