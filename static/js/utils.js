/**
 * @param {number} x
 */
export const likesMsg = x => `${x} like${x !== 1 ? 's' : ''}`

/**
 * @param {number} x
 */
export const commentsMsg = x => `${x} comment${x !== 1 ? 's' : ''}`

/**
 * @param {number} x
 */
export const followersMsg = x => `${x} follower${x !== 1 ? 's' : ''}`

/**
 * @param {boolean} x
 */
export const followMsg = x => x ? 'Following' : 'Follow'

export const isObject = x => typeof x === 'object' && x !== null

/**
 * Goes to the given URL.
 *
 * @param {string} url
 * @param {boolean} replace
 */
export function goto(url, replace = false) {
    const { state } = history
    history[`${replace ? 'replace' : 'push'}State`](state, document.title, url)
    dispatchEvent(new PopStateEvent('popstate', { state }))
}

/**
 * Removes empty lines and extra spaces.
 * @param {string} content
 */
export const sanitizeContent = content => content
    .split('\n')
    .map(line => line.trim())
    .filter(line => line !== '')
    .map(line => line.replace(/\s+/, ' '))
    .join('\n')

/**
 * Escapes HTML.
 * @param {string} html
 */
export const escapeHTML = html => html
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')

const rxURL = new RegExp('(?:(?:(?:[a-z]+:)?//)|www\\.)(?:localhost|(?:(?:[a-z\\u00a1-\\uffff0-9]-*)*[a-z\\u00a1-\\uffff0-9]+)(?:\\.(?:[a-z\\u00a1-\\uffff0-9]-*)*[a-z\\u00a1-\\uffff0-9]+)*)(?:[/?#][^\\s"]*)?', 'ig')

/**
 * Parses links.
 * @param {string} content
 */
export const linkify = content => content
    .replace(rxURL, url => `<a href="${url}" target="_blank" rel="noopener noreferrer">${decodeURI(url)}</a>`)

/**
 * Wraps spoileable content.
 * @param {string=} spoilerOf
 * @param {string} content
 */
export const wrapInSpoiler = (spoilerOf, content) => spoilerOf !== null ? `
    <div class="spoiler-wrapper">
        <p>This post contains spoilers of: ${escapeHTML(spoilerOf)}</p>
        <button class="spoiler-toggler">Show</button>
    </div>
    <div class="content" hidden>${content}</div>
` : content

const MONTHS = [
    'Jan',
    'Feb',
    'Mar',
    'Apr',
    'May',
    'Jun',
    'Jul',
    'Aug',
    'Sep',
    'Oct',
    'Nov',
    'Dec'
]

const SECONDS = {
    MINUTE: 60,
    HOUR: 3600,
    DAY: 86400,
    MONTH: 2592000,
    YEAR: 31536000
}

/**
 * Adds a zero before.
 *
 * @param {Date} date
 * @returns {string}
 */
const formatDay = date => String(date.getDate()).padStart(2, '0')

/**
 * Formats date in "ago" format.
 *
 * @param {string|Date} x
 * @returns {string}
 */
export const ago = x => {
    const date = x instanceof Date ? x : new Date(x)
    // @ts-ignore
    const secondsAgo = Math.floor((new Date() - date) / 1000)

    let interval = Math.floor(secondsAgo / SECONDS.YEAR)
    if (interval >= 1) return `${formatDay(date)} ${MONTHS[date.getMonth()]}, ${date.getFullYear()}`

    interval = Math.floor(secondsAgo / SECONDS.MONTH)
    if (interval >= 1) return `${formatDay(date)} ${MONTHS[date.getMonth()]}`

    interval = Math.floor(secondsAgo / SECONDS.DAY)
    if (interval === 1) return 'Yesterday'
    if (interval > 1) return interval + 'd'

    interval = Math.floor(secondsAgo / SECONDS.HOUR)
    if (interval >= 1) return interval + 'h'

    interval = Math.floor(secondsAgo / SECONDS.MINUTE)
    if (interval >= 1) return interval + 'm'

    return secondsAgo === 0 ? 'Just now' : secondsAgo + 's'
}

/**
 * Builds a user friendly notification message.
 *
 * @returns {string=}
 */
export function getNotificationMessage({ actorUsername, verb, objectId, targetId }) {
    switch (verb) {
        case 'follow': return actorUsername + ' followed you'
        case 'post_mention': return actorUsername + ' mentioned you in a post'
        case 'comment': return actorUsername + ' commented on a post'
        case 'comment_mention': return actorUsername + ' mentioned you in a comment'
    }
    return null
}

/**
 * Builds the href to notification action.
 */
export function getNotificationHref({ actorUsername, verb, objectId, targetId }) {
    switch (verb) {
        case 'follow': return '/users/' + actorUsername
        case 'post_mention': return '/posts/' + objectId
        case 'comment':
        case 'comment_mention': return `/posts/${targetId}#comment-${objectId}`
    }
    return '#!'
}
