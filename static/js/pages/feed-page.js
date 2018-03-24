import { likeable, spoileable } from '../behaviors.js'
import http from '../http.js'
import { ago, avatarImg, commentsMsg, escapeHTML, likesMsg, linkify, sanitizeContent, wrapInSpoiler } from '../utils.js'

const template = document.createElement('template')
template.innerHTML = `
<div class="container">
    <h1>Feed</h1>
    <form id="post-form">
        <textarea placeholder="Write something..."  maxlength="480" required></textarea>
        <label>
            <input type="checkbox"> Spoiler
        </label>
        <input type="text" placeholder="Spoiler of..." maxlength="128" hidden>
        <button type="submit">Post</button>
    </form>
    <button id="flush-queue" hidden></button>
    <div id="feed" class="articles" role="feed"></div>
    <button id="load-more" hidden>Load more</button>
</div>
`

let lastFeedItemId
const feedQueue = []
const feedCache = []

function addToCache(feed) {
    feedCache.push(...feed)
    return feed
}

function saveLastItemId(feed) {
    const l = feed.length
    if (l !== 0) {
        lastFeedItemId = feed[l - 1]['id']
    }
    return feed
}

const getFeed = () => feedCache.length !== 0
    ? Promise.resolve(feedCache)
    : http.get('/api/feed').then(addToCache).then(saveLastItemId)

const loadMore = () => typeof lastFeedItemId === 'undefined'
    ? Promise.resolve([])
    : http.get('/api/feed?before=' + lastFeedItemId).then(addToCache).then(saveLastItemId)

function createFeedItemArticle(feedItem) {
    const { post } = feedItem
    const { user } = post
    const createdAt = ago(post.createdAt)
    const content = linkify(escapeHTML(post.content))

    const article = document.createElement('article')
    article.innerHTML = wrapInSpoiler(post.spoilerOf, `
        <header>
            <a href="/users/${user.username}">
                ${avatarImg(user)}
                <span>${user.username}</span>
            </a>
            <a href="/posts/${post.id}" class="created-at"><time>${createdAt}</time></a>
        </header>
        <p style="white-space: pre">${content}</p>
        <div>
            <button role="switch" class="likes-count${post.liked ? ' liked' : ''}" aria-label="${likesMsg(post.likesCount)}" aria-checked="${post.liked}">${post.likesCount}</button>
            <a class="comments-count" href="/posts/${post.id}" title="${commentsMsg(post.commentsCount)}">${post.commentsCount}</a>
        </div>
    `)

    if (post.spoilerOf !== null) {
        spoileable(article.querySelector('.spoiler-toggler'))
    }

    likeable(article.querySelector('.likes-count'), `posts/${post.id}`)

    return article
}

export default function () {
    const page = /** @type {DocumentFragment} */ (template.content.cloneNode(true))
    const postForm = /** @type {HTMLFormElement} */ (page.getElementById('post-form'))
    const postTextArea = postForm.querySelector('textarea')
    const postSpoilerCheckbox = /** @type {HTMLInputElement} */ (postForm.querySelector('input[type=checkbox]'))
    const postSpoilerInput = /** @type {HTMLInputElement} */ (postForm.querySelector('input[type=text]'))
    const postButton = postForm.querySelector('button')
    const flushQueueButton = page.getElementById('flush-queue')
    const feedDiv = page.getElementById('feed')
    const loadMoreButton = /** @type {HTMLButtonElement} */ (page.getElementById('load-more'))

    postForm.addEventListener('submit', ev => {
        ev.preventDefault()
        const content = sanitizeContent(postTextArea.value)
        const isSpoiler = postSpoilerCheckbox.checked
        const spoilerOf = postSpoilerInput.value.trim()

        if (content === '') {
            postTextArea.setCustomValidity('Empty')
            return
        }
        if (isSpoiler && spoilerOf === '') {
            postSpoilerInput.setCustomValidity('Empty')
            return
        }

        const payload = { content }
        if (isSpoiler) {
            payload['spoilerOf'] = spoilerOf
        }

        postTextArea.disabled = true
        postButton.disabled = true

        http.post('/api/posts', payload).then(feedItem => {
            flushQueue()
            feedDiv.insertBefore(createFeedItemArticle(feedItem), feedDiv.firstChild)
            postForm.reset()
            postTextArea.setCustomValidity('')
            postSpoilerInput.setCustomValidity('')
            postSpoilerCheckbox.checked = false
            postSpoilerInput.hidden = true
            postSpoilerInput.required = false
        }).catch(err => {
            console.error(err)
            alert(err.message)
            postTextArea.focus()
        }).then(() => {
            postTextArea.disabled = false
            postButton.disabled = false
        })
    })

    postTextArea.addEventListener('input', () => {
        postTextArea.setCustomValidity('')
    })

    postSpoilerCheckbox.addEventListener('change', () => {
        if (postSpoilerCheckbox.checked) {
            postSpoilerInput.hidden = false
            postSpoilerInput.required = true
        } else {
            postSpoilerInput.hidden = true
            postSpoilerInput.required = false
        }
    })

    postSpoilerInput.addEventListener('input', () => {
        postSpoilerInput.setCustomValidity('')
    })

    const flushQueue = () => {
        let feedItem
        while (feedItem = feedQueue.shift()) {
            feedDiv.insertBefore(createFeedItemArticle(feedItem), feedDiv.firstChild)
        }
        flushQueueButton.hidden = true
    }

    flushQueueButton.addEventListener('click', flushQueue)

    getFeed().then(feed => {
        feed.forEach(feedItem => {
            feedDiv.appendChild(createFeedItemArticle(feedItem))
        })
        loadMoreButton.hidden = false
    }).catch(console.error)

    loadMoreButton.addEventListener('click', () => {
        loadMoreButton.disabled = true
        loadMore()
            .then(feed => {
                feed.forEach(feedItem => {
                    feedDiv.appendChild(createFeedItemArticle(feedItem))
                })
                return feed
            })
            .catch(console.error)
            .then(feed => {
                if (feed.length < 25) {
                    loadMoreButton.hidden = true
                    return
                }
                loadMoreButton.disabled = false
            })
    })

    const unsubscribe = http.subscribe('/api/feed', feedItem => {
        feedQueue.push(feedItem)
        feedCache.unshift(feedItem)
        flushQueueButton.hidden = false
        const l = feedQueue.length
        flushQueueButton.textContent = `${l} new post${l !== 1 ? 's' : ''}`
    })

    page.addEventListener('disconnect', unsubscribe)

    return page
}
