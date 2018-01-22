import { authenticated } from './auth.js'
import html from './html.js'
import http from './http.js'
import { followable } from './behaviors.js'
import { followersMsg, followMsg } from './utils.js'

function createUserArticle(user) {
    const article = document.createElement('article')
    article.className = 'user'
    article.innerHTML = `
        <a href="/users/${user.username}">
            <figure class="avatar" data-initial="${user.username[0]}"></figure>
            <span>${user.username}</span>
        </a>
        <div class="user-stats">
            <span class="followers-count">${followersMsg(user.followersCount)}</span>
            <span>${user.followingCount} following</span>
        </div>
        ${authenticated ? `
            <div>
                <button class="follow">${followMsg(user.followingOfMine)}</button>
            </div>
        ` : ''}
    `

    if (authenticated) {
        followable(article.querySelector('.follow'), user.username)
    }

    return article
}

/**
 * Fills a list with users.
 *
 * @param {Node} el
 * @param {any[]} users
 */
export default function (el, users) {
    users.forEach(user => {
        el.appendChild(createUserArticle(user))
    })
}
