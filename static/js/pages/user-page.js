import { authenticated } from '../auth.js'
import http from '../http.js'
import html from '../html.js'
import { likeable, followable, spoileable } from '../behaviors.js'
import { followersMsg, followMsg, likesMsg, commentsMsg, goto, linkify, escapeHTML, wrapInSpoiler, ago } from '../utils.js'

const template = html`
<div class="profile-wrapper"></div>
<div id="posts" class="container articles" role="feed"></div>
`

function createPostArticle(post) {
    const { user } = post
    const createdAt = ago(post.createdAt)
    const content = linkify(escapeHTML(post.content))

    const article = document.createElement('article')
    article.innerHTML = wrapInSpoiler(post.spoilerOf, `
        <header>
            <div>
                <figure class="avatar" data-initial="${user.username[0]}"></figure>
                <span>${user.username}</span>
            </div>
            <a href="/posts/${post.id}" class="created-at"><time>${createdAt}</time></a>
        </header>
        <p>${content}</p>
        <div>
            <${authenticated ? 'button role="switch"' : 'span'} class="likes-count${post.liked ? ' liked' : ''}" aria-label="${likesMsg(post.likesCount)}"${authenticated ? ` aria-checked="${post.liked}"` : ''}>${post.likesCount}</${authenticated ? 'button' : 'span'}>
            <a class="comments-count" href="/posts/${post.id}" title="${commentsMsg(post.commentsCount)}">${post.commentsCount}</a>
        </div>
    `)

    if (post.spoilerOf !== null) {
        spoileable(article.querySelector('.spoiler-toggler'))
    }

    if (authenticated) {
        likeable(article.querySelector('.likes-count'), `posts/${post.id}`)
    }

    return article
}

export default function (username) {
    const page = /** @type {DocumentFragment} */ (template.content.cloneNode(true))
    const profileDiv = page.querySelector('.profile-wrapper')
    const postsDiv = page.getElementById('posts')

    Promise.all([
        http.get('/api/users/' + username),
        http.get(`/api/users/${username}/posts`)
    ]).then(([user, posts]) => {
        profileDiv.innerHTML = `
            <div class="container">
                <div>
                    <figure class="avatar big" data-initial="${user.username[0]}"></figure>
                    <h1>${user.username}</h1>
                </div>
                <div class="user-stats">
                <a href="/users/${user.username}/followers" class="followers-count">${followersMsg(user.followersCount)}</a>
                <a href="/users/${user.username}/following">${user.followingCount} following</a>
                </div>
                <div>
                    ${user.me ? `
                        <button>Edit avatar</button>
                        <input type="file" accept="image/jpg,image/png" hidden>
                        <button id="logout">Logout</button>
                    ` : authenticated ? `
                        <button id="follow" title="${followMsg(user.followingOfMine)}">${followMsg(user.followingOfMine)}</button>
                    ` : ''}
                </div>
            </div>
        `

        if (user.me) {
            profileDiv.querySelector('#logout').addEventListener('click', () => {
                localStorage.clear()
                // Clear cookies
                document.cookie.split(';').forEach(c => {
                    document.cookie = c.trim()
                        .replace(/=.*/, `=;expires=${new Date().toUTCString()};path=/`)
                })
                location.assign('/')
            })
        } else if (authenticated) {
            followable(profileDiv.querySelector('#follow'), user.username)
        }

        posts.forEach(post => {
            post['user'] = user
            postsDiv.appendChild(createPostArticle(post))
        })
    }).catch(err => {
        console.error(err)
        if (err.statusCode === 404) {
            goto('/not-found', true)
        }
    })

    return page
}
