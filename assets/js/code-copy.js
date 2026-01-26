// Code copy buttons for code blocks
document.querySelectorAll('pre > code').forEach(function(codeblock) {
    var container = codeblock.parentNode.parentNode;

    var copybutton = document.createElement('button');
    copybutton.classList.add('copy-code');
    copybutton.type = 'button';
    copybutton.setAttribute('aria-label', 'Copy code to clipboard');
    copybutton.innerHTML = window.codeCopyText || 'copy';

    function copyingDone() {
        copybutton.innerHTML = window.codeCopiedText || 'copied!';
        copybutton.setAttribute('aria-label', 'Code copied to clipboard');
        // Announce to screen readers
        var announcer = document.getElementById('sr-announcements');
        if (announcer) {
            announcer.textContent = 'Code copied to clipboard';
        }
        setTimeout(function() {
            copybutton.innerHTML = window.codeCopyText || 'copy';
            copybutton.setAttribute('aria-label', 'Copy code to clipboard');
        }, 2000);
    }

    copybutton.addEventListener('click', function() {
        if ('clipboard' in navigator) {
            navigator.clipboard.writeText(codeblock.textContent);
            copyingDone();
            return;
        }

        var range = document.createRange();
        range.selectNodeContents(codeblock);
        var selection = window.getSelection();
        selection.removeAllRanges();
        selection.addRange(range);
        try {
            document.execCommand('copy');
            copyingDone();
        } catch (e) {}
        selection.removeRange(range);
    });

    if (container.classList.contains("highlight")) {
        container.appendChild(copybutton);
    } else if (container.parentNode.firstChild == container) {
        // td containing LineNos - skip
    } else if (codeblock.parentNode.parentNode.parentNode.parentNode.parentNode.nodeName == "TABLE") {
        // table containing LineNos and code
        codeblock.parentNode.parentNode.parentNode.parentNode.parentNode.appendChild(copybutton);
    } else {
        // code blocks not having highlight as parent class
        codeblock.parentNode.appendChild(copybutton);
    }
});
