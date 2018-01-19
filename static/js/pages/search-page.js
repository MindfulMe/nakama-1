import { authenticated } from '../auth.js'
import http from '../http.js'
import html from '../html.js'
import { goto, followersMsg, followMsg } from '../utils.js'
import { followable } from '../behaviors.js'

const template = html`
<div class="container">
    <h1>Search</h1>
    <form id="search">
        <input type="search" placeholder="Search..." autofocus required>
        <button type="submit">Search</button>
    </form>
    <div id="results" class="articles"></div>
</div>
`

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

export default function () {
    const page = /** @type {DocumentFragment} */ (template.content.cloneNode(true))
    const searchForm = /** @type {HTMLFormElement} */ (page.getElementById('search'))
    const searchInput = searchForm.querySelector('input')
    const searchButton = searchForm.querySelector('button')
    const resultDiv = page.getElementById('results')

    searchForm.addEventListener('submit', ev => {
        ev.preventDefault()
        const username = searchInput.value.trim()
        searchInput.disabled = true
        searchButton.disabled = true
        http.get('/api/users?username=' + username).then(users => {
            if (users.length === 1) {
                goto('/users/' + users[0].username)
                return
            }
            users.forEach(user => {
                resultDiv.appendChild(createUserArticle(user))
            })
        }).catch(err => {
            console.error(err)
            alert(err.message)
            searchInput.focus()
        }).then(() => {
            searchInput.disabled = false
            searchButton.disabled = false
        })
    })

    return page
}
