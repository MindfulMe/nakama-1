import { getAuthUser } from '../auth.js'
import { followable, likeable, spoileable } from '../behaviors.js'
import http from '../http.js'
import { ago, avatarImg, commentsMsg, escapeHTML, followMsg, followersMsg, goto, likesMsg, linkify, wrapInSpoiler } from '../utils.js'

const authenticated = getAuthUser() !== null

const template = document.createElement('template')
template.innerHTML = `
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
                ${avatarImg(user)}
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
                    ${avatarImg(user, true)}
                    <h1>${user.username}</h1>
                </div>
                <div class="user-stats">
                <a href="/users/${user.username}/followers" class="followers-count">${followersMsg(user.followersCount)}</a>
                <a href="/users/${user.username}/following">${user.followingCount} following</a>
                </div>
                <div>
                    ${user.me ? `
                        <button id="avatar-button">Edit avatar</button>
                        <input id="avatar-input" type="file" accept="image/jpg,image/png" hidden>
                        <button id="logout">Logout</button>
                    ` : authenticated ? `
                        <button id="follow" title="${followMsg(user.followingOfMine)}">${followMsg(user.followingOfMine)}</button>
                    ` : ''}
                </div>
            </div>
        `

        if (user.me) {
            profileDiv.querySelector('#logout').addEventListener('click', () => {
                http.post('/api/logout').then(() => {
                    localStorage.clear()
                    location.assign('/')
                }).catch(err => {
                    console.error(err)
                    alert('could not logout')
                })
            })

            const avatarButton = /** @type {HTMLButtonElement} */ (profileDiv.querySelector('#avatar-button'))
            const avatarInput = /** @type {HTMLInputElement} */ (profileDiv.querySelector('#avatar-input'))
            avatarButton.addEventListener('click', () => {
                avatarInput.click()
            })
            avatarInput.addEventListener('change', () => {
                const avatar = avatarInput.files[0]
                avatarButton.disabled = true
                avatarInput.disabled = true
                http.post('/api/upload_avatar', avatar, { 'Content-Type': avatar.type }).then(avatarUrl => {
                    const authUser = JSON.parse(localStorage.getItem('auth_user'))
                    authUser.avatarUrl = avatarUrl
                    localStorage.setItem('auth_user', JSON.stringify(authUser))
                    location.reload()
                }).catch(err => {
                    console.error(err)
                    alert(err.message)
                    avatarButton.disabled = false
                    avatarInput.disabled = false
                })
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
