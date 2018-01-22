import http from '../http.js'
import html from '../html.js'
import usersList from '../users-list.js'

const template = html`
<div class="container">
    <h1>Following</h1>
    <div id="results" class="articles"></div>
</div>
`

export default function (username) {
    const page = /** @type {DocumentFragment} */ (template.content.cloneNode(true))
    const title = page.querySelector('h1')
    const resultsDiv = page.getElementById('results')

    title.textContent = `${username}'s following`

    http.get(`/api/users/${username}/following`).then(users => {
        usersList(resultsDiv, users)
    })

    return page
}
