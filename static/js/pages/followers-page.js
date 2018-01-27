import http from '../http.js'
import usersList from '../users-list.js'

const template = document.createElement('template')
template.innerHTML = `
<div class="container">
    <h1>Followers</h1>
    <div id="results" class="articles"></div>
</div>
`

export default function (username) {
    const page = /** @type {DocumentFragment} */ (template.content.cloneNode(true))
    const title = page.querySelector('h1')
    const resultsDiv = page.getElementById('results')

    title.textContent = `${username}'s followers`

    http.get(`/api/users/${username}/followers`).then(users => {
        usersList(resultsDiv, users)
    })

    return page
}
