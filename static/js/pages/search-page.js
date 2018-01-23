import { getAuthUser } from '../auth.js'
import http from '../http.js'
import html from '../html.js'
import { goto } from '../utils.js'
import usersList from '../users-list.js'

const authenticated = getAuthUser() !== null

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

export default function () {
    const page = /** @type {DocumentFragment} */ (template.content.cloneNode(true))
    const searchForm = /** @type {HTMLFormElement} */ (page.getElementById('search'))
    const searchInput = searchForm.querySelector('input')
    const searchButton = searchForm.querySelector('button')
    const resultDiv = page.getElementById('results')

    searchInput.focus()

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
            usersList(resultDiv, users)
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
