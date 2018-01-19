import http from '../http.js'
import html from '../html.js'
import { ago } from '../utils.js'

const template = html`
<div class="container">
    <h1>Notifications</h1>
    <div id="notifications" class="notifications"></div>
</div>
`

function createNotificationLink(notification) {
    let action = ''
    const a = document.createElement('a')
    a.className = 'notification'
    if (notification.read) {
        a.classList.add('read')
    }
    switch (notification.verb) {
        case 'follow':
            action = 'followed you'
            a.href = '/users/' + notification.actorUsername
            break
        case 'post_mention':
            action = 'mentioned you in a post'
            a.href = '/posts/' + notification.objectId
            break
        case 'comment':
            action = 'commented on a post'
            a.href = `/posts/${notification.targetId}#comment-${notification.objectId}`
            break
        case 'comment_mention':
            action = 'mentioned you in a comment'
            a.href = `/posts/${notification.targetId}#comment-${notification.objectId}`
            break
    }
    a.innerHTML = `
        <span>${notification.actorUsername} ${action}</span>
        <time>${ago(notification.issuedAt)}</time>
    `
    return a
}

export default function () {
    const page = /** @type {DocumentFragment} */ (template.content.cloneNode(true))
    const notificationsDiv = page.getElementById('notifications')
    const notificationsLink = document.querySelector('#notifications-link.unread')

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
