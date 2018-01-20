import http from '../http.js'
import html from '../html.js'
import { ago, getNotificationMessage, getNotificationHref } from '../utils.js'

const template = html`
<div class="container">
    <h1>Notifications</h1>
    <div id="notifications" class="notifications"></div>
</div>
`

function createNotificationLink(notification) {
    const a = document.createElement('a')
    const message = getNotificationMessage(notification)
    if (message === null) {
        a.hidden = true
        return a
    }
    a.className = 'notification'
    if (notification.read) {
        a.classList.add('read')
    }
    a.href = getNotificationHref(notification)
    a.innerHTML = `
        <span>${message}</span>
        <time>${ago(notification.issuedAt)}</time>
    `
    return a
}

export default function () {
    const page = /** @type {DocumentFragment} */ (template.content.cloneNode(true))
    const notificationsLink = document.querySelector('#notifications-link.unread')
    const notificationsDiv = page.getElementById('notifications')

    if (notificationsLink !== null) {
        notificationsLink.classList.remove('unread')
    }

    http.get('/api/notifications').then(notifications => {
        notifications.forEach(notification => {
            notificationsDiv.appendChild(createNotificationLink(notification))
        })
    }).catch(console.error)

    return page
}
