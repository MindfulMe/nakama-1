import { getAuthUser } from './auth.js'

const authenticated = getAuthUser() !== null
const pagesCache = new Map()

/**
 * Imports a page and puts it in cache.
 *
 * @param {string} name
 * @returns {function(...string): Promise<Node>}
 */
const genPage = name => async (...args) => {
    if (pagesCache.has(name)) {
        const page = pagesCache.get(name)
        return page(...args)
    }
    const page = await import(`/js/pages/${name}-page.js`)
        .then(m => m.default)
    pagesCache.set(name, page)
    return page(...args)
}

/**
 * Router.
 *
 * @param {array} routes
 * @returns {function(string): Promise<Node>}
 */
const router = routes => pathname => {
    for (const [pattern, fn] of routes) {
        if (typeof pattern === 'string') {
            if (pattern !== pathname) continue
            return fn()
        }
        const match = pattern.exec(pathname)
        if (match === null) continue
        return fn(...match.slice(1))
    }
}

const notFound = genPage('not-found')

/**
 * Route definitions.
 */
const route = router([
    ['/', authenticated ? genPage('feed') : genPage('welcome')],
    ['/callback', genPage('callback')],
    ['/search', genPage('search')],
    ['/notifications', authenticated ? genPage('notifications') : notFound],
    [/^\/users\/([^\/]+)\/following$/, genPage('following')],
    [/^\/users\/([^\/]+)\/followers$/, genPage('followers')],
    [/^\/users\/([^\/]+)$/, genPage('user')],
    [/^\/posts\/([^\/]+)$/, genPage('post')],
    [/^\//, notFound],
])

/** @type {Node} */ let currentPage
const disconnectEvent = new CustomEvent('disconnect')
const pageOutlet = /** @type {HTMLDivElement} */ (document.getElementById('page'))

/**
 * Renders a page based on the current url.
 */
async function render() {
    if (currentPage instanceof Node) {
        currentPage.dispatchEvent(disconnectEvent) // Announces when the user leaves the page
        pageOutlet.innerHTML = ''
    }
    currentPage = await route(decodeURI(location.pathname))
    pageOutlet.appendChild(currentPage)
}

/**
 * Intercept clicks
 * and if it is on an anchor element prevent it
 * and dispatch a popstate event.
 *
 * @param {MouseEvent} ev
 */
function hijackClicks(ev) {
    if (ev.defaultPrevented
        || ev.ctrlKey
        || ev.metaKey
        || ev.altKey
        || ev.shiftKey
        || ev.button !== 0) return

    let currentTarget = /** @type {Node} */ (ev.target)
    do {
        if (currentTarget.nodeName.toUpperCase() !== 'A') continue
        const a = /** @type {HTMLAnchorElement} */ (currentTarget)
        if (a.target !== '' && !/^_?self$/i.test(a.target)) continue

        ev.stopImmediatePropagation()
        ev.stopPropagation()
        ev.preventDefault()

        const { state } = history
        history.pushState(state, document.title, a.href)
        dispatchEvent(new PopStateEvent('popstate', { state }))

        return false
    } while (currentTarget.parentNode !== null && (currentTarget = currentTarget.parentNode))
}

render() // Initial render
addEventListener('popstate', render) // Re-render on browser history
addEventListener('click', hijackClicks) // Re-render on anchor link clicks
