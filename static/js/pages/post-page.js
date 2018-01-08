import { authenticated } from '../auth.js'
import http from '../http.js'
import { likeable } from '../behaviors.js'
import { goto, likesMsg, commentsMsg, sanitizeContent, linkify, escapeHTML, ago } from '../utils.js'

const template = document.createElement('template')
template.innerHTML = `
<div class="post-wrapper"></div>
<div class="container">
    <div id="comments" class="articles" role="feed"></div>
    <form id="comment-form" hidden>
        <textarea placeholder="Comment something..." required></textarea>
        <button type="submit">Comment</button>
    </form>
</div>
`

function createCommentArticle(comment) {
    const { user } = comment
    const createdAt = ago(comment.createdAt)
    const content = linkify(escapeHTML(comment.content))

    const article = document.createElement('article')
    article.innerHTML = `
        <header>
            <a href="/users/${user.username}">
                <figure class="avatar" data-initial="${user.username[0]}"></figure>
                <span>${user.username}</span>
            </a>
            <time class="created-at">${createdAt}</time>
        </header>
        <p>${content}</p>
        <div>
            <${authenticated ? 'button' : 'span'} class="likes-count${comment.liked ? ' liked' : ''}" title="${likesMsg(comment.likesCount)}">${comment.likesCount}</${authenticated ? 'button' : 'span'}>
        </div>
    `

    if (authenticated) {
        likeable(article.querySelector('.likes-count'), `comments/${comment.id}`)
    }

    return article
}

const subscribeMsg = x => x ? 'Mute' : 'Subscribe'

export default function (postId) {
    const page = /** @type {DocumentFragment} */ (template.content.cloneNode(true))
    const postDiv = page.querySelector('.post-wrapper')
    const commentsDiv = page.getElementById('comments')
    const commentForm = /** @type {HTMLFormElement} */ (page.getElementById('comment-form'))
    const commentTextArea = commentForm.querySelector('textarea')
    const commentButton = commentForm.querySelector('button')
    let commentsCountSpan = /** @type {HTMLSpanElement} */ (null)
    let subscribeButton = /** @type {HTMLButtonElement} */ (null)

    Promise.all([
        http.get('/api/posts/' + postId),
        http.get(`/api/posts/${postId}/comments`)
    ]).then(([post, comments]) => {
        const { user } = post
        const createdAt = ago(post.createdAt)
        const content = linkify(escapeHTML(post.content))

        postDiv.innerHTML = `
            <article class="container">
                <header>
                    <a href="/users/${user.username}">
                        <figure class="avatar" data-initial="${user.username[0]}"></figure>
                        <span>${user.username}</span>
                    </a>
                    <time class="created-at">${createdAt}</time>
                </header>
                <p>${content}</p>
                <div>
                    <${authenticated ? 'button' : 'span'} class="likes-count${post.liked ? ' liked' : ''}" title="${likesMsg(post.likesCount)}">${post.likesCount}</${authenticated ? 'button' : 'span'}>
                    <span class="comments-count" title="${commentsMsg(post.commentsCount)}">${post.commentsCount}</span>
                    ${authenticated ? `
                        <button id="subscribe">${subscribeMsg(post.subscribed)}</button>
                    ` : ''}
                </div>
            </article>
        `

        commentsCountSpan = postDiv.querySelector('.comments-count')

        if (authenticated) {
            likeable(postDiv.querySelector('.likes-count'), `posts/${post.id}`)

            subscribeButton = postDiv.querySelector('#subscribe')
            subscribeButton.addEventListener('click', () => {
                subscribeButton.disabled = true
                http.post(`/api/posts/${post.id}/toggle_subscription`).then(subscribed => {
                    subscribeButton.textContent = subscribeMsg(subscribed)
                }).catch(console.error).then(() => {
                    subscribeButton.disabled = false
                })
            })
        }

        comments.forEach(comment => {
            commentsDiv.insertBefore(createCommentArticle(comment), commentsDiv.firstChild)
        })
    }).catch(err => {
        console.error(err)
        if (err.statusCode === 404) {
            goto('/not-found', true)
        }
    })

    if (authenticated) {
        commentForm.hidden = false
        commentForm.addEventListener('submit', ev => {
            ev.preventDefault()
            const content = sanitizeContent(commentTextArea.value)

            if (content === '') {
                commentTextArea.setCustomValidity('Empty')
                return
            }

            commentTextArea.disabled = true
            commentButton.disabled = true

            http.post(`/api/posts/${postId}/comments`, { content }).then(comment => {
                commentsDiv.appendChild(createCommentArticle(comment))
                commentForm.reset()
                commentTextArea.setCustomValidity('')
                if (commentsCountSpan !== null) {
                    const oldCount = parseInt(commentsCountSpan.textContent, 10)
                    commentsCountSpan.textContent = String(oldCount + 1)
                }
                if (subscribeButton !== null) {
                    subscribeButton.textContent = subscribeMsg(true)
                }
            }).catch(err => {
                console.error(err)
                alert(err.message)
                commentTextArea.focus()
            }).then(() => {
                commentTextArea.disabled = false
                commentButton.disabled = false
            })
        })

        commentTextArea.addEventListener('input', () => {
            commentTextArea.setCustomValidity('')
        })
    }

    return page
}
