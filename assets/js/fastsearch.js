import * as params from '@params';

let fuse; // holds our search engine
let resList = document.getElementById('searchResults');
let sInput = document.getElementById('searchInput');
let first, last, current_elem = null
let resultsAvailable = false;
let indexLoaded = false;
let indexLoading = false;

// Lazy load search index - only fetch when user interacts with search
function loadSearchIndex() {
    if (indexLoaded || indexLoading) return;

    indexLoading = true;

    // Show loading indicator
    if (resList) {
        resList.innerHTML = '<li class="post-entry">Loading search index...</li>';
    }

    let xhr = new XMLHttpRequest();
    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            if (xhr.status === 200) {
                let data = JSON.parse(xhr.responseText);
                if (data) {
                    // fuse.js options; check fuse.js website for details
                    let options = {
                        distance: 100,
                        threshold: 0.4,
                        ignoreLocation: true,
                        keys: [
                            'title',
                            'permalink',
                            'summary',
                            'content'
                        ]
                    };
                    if (params.fuseOpts) {
                        options = {
                            isCaseSensitive: params.fuseOpts.iscasesensitive ?? false,
                            includeScore: params.fuseOpts.includescore ?? false,
                            includeMatches: params.fuseOpts.includematches ?? false,
                            minMatchCharLength: params.fuseOpts.minmatchcharlength ?? 1,
                            shouldSort: params.fuseOpts.shouldsort ?? true,
                            findAllMatches: params.fuseOpts.findallmatches ?? false,
                            keys: params.fuseOpts.keys ?? ['title', 'permalink', 'summary', 'content'],
                            location: params.fuseOpts.location ?? 0,
                            threshold: params.fuseOpts.threshold ?? 0.4,
                            distance: params.fuseOpts.distance ?? 100,
                            ignoreLocation: params.fuseOpts.ignorelocation ?? true
                        }
                    }
                    fuse = new Fuse(data, options); // build the index from the json file
                    indexLoaded = true;
                    indexLoading = false;

                    // Clear loading message and trigger search if there's already input
                    if (resList) {
                        resList.innerHTML = '';
                    }
                    if (sInput && sInput.value.trim()) {
                        executeSearch(sInput.value.trim());
                    }
                }
            } else {
                console.log(xhr.responseText);
                indexLoading = false;
                if (resList) {
                    resList.innerHTML = '<li class="post-entry">Failed to load search index</li>';
                }
            }
        }
    };
    xhr.open('GET', "../index.json");
    xhr.send();
}

function activeToggle(ae) {
    document.querySelectorAll('.focus').forEach(function (element) {
        // rm focus class
        element.classList.remove("focus")
    });
    if (ae) {
        ae.focus()
        document.activeElement = current_elem = ae;
        ae.parentElement.classList.add("focus")
    } else {
        document.activeElement.parentElement.classList.add("focus")
    }
}

function reset() {
    resultsAvailable = false;
    resList.innerHTML = sInput.value = ''; // clear inputbox and searchResults
    sInput.focus(); // shift focus to input box
}

function executeSearch(searchValue) {
    if (!fuse) return;

    let results;
    if (params.fuseOpts) {
        results = fuse.search(searchValue, {limit: params.fuseOpts.limit}); // the actual query being run using fuse.js along with options
    } else {
        results = fuse.search(searchValue); // the actual query being run using fuse.js
    }

    if (results.length !== 0) {
        // build our html if result exists
        let resultSet = ''; // our results bucket

        for (let item in results) {
            resultSet += `<li class="post-entry"><header class="entry-header">${results[item].item.title}&nbsp;»</header>` +
                `<a href="${results[item].item.permalink}" aria-label="${results[item].item.title}"></a></li>`
        }

        resList.innerHTML = resultSet;
        resultsAvailable = true;
        first = resList.firstChild;
        last = resList.lastChild;
    } else {
        resultsAvailable = false;
        resList.innerHTML = '';
    }
}

// Load index on first interaction with search input
if (sInput) {
    sInput.addEventListener('focus', function() {
        loadSearchIndex();
    }, { once: false });

    sInput.addEventListener('input', function() {
        loadSearchIndex();
    }, { once: true });
}

// execute search as each character is typed
if (sInput) {
    sInput.onkeyup = function (e) {
        // Trigger index load if not already loaded
        if (!indexLoaded) {
            loadSearchIndex();
            return;
        }

        // run a search query (for "term") every time a letter is typed
        // in the search box
        executeSearch(this.value.trim());
    }
}

if (sInput) {
    sInput.addEventListener('search', function (e) {
        // clicked on x
        if (!this.value) reset()
    })
}

// kb bindings
document.onkeydown = function (e) {
    let key = e.key;
    let ae = document.activeElement;

    let inbox = document.getElementById("searchbox").contains(ae)

    if (ae === sInput) {
        let elements = document.getElementsByClassName('focus');
        while (elements.length > 0) {
            elements[0].classList.remove('focus');
        }
    } else if (current_elem) ae = current_elem;

    if (key === "Escape") {
        reset()
    } else if (!resultsAvailable || !inbox) {
        return
    } else if (key === "ArrowDown") {
        e.preventDefault();
        if (ae == sInput) {
            // if the currently focused element is the search input, focus the <a> of first <li>
            activeToggle(resList.firstChild.lastChild);
        } else if (ae.parentElement != last) {
            // if the currently focused element's parent is last, do nothing
            // otherwise select the next search result
            activeToggle(ae.parentElement.nextSibling.lastChild);
        }
    } else if (key === "ArrowUp") {
        e.preventDefault();
        if (ae.parentElement == first) {
            // if the currently focused element is first item, go to input box
            activeToggle(sInput);
        } else if (ae != sInput) {
            // if the currently focused element is input box, do nothing
            // otherwise select the previous search result
            activeToggle(ae.parentElement.previousSibling.lastChild);
        }
    } else if (key === "ArrowRight") {
        ae.click(); // click on active link
    }
}
