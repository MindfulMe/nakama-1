import http from '../http.js'
import html from '../html.js'
import { ago, getNotificationMessage, getNotificationHref } from '../utils.js'

const template = html`
<div class="container">
    <h1>Notifications</h1>
    <button id="flush-queue" hidden></button>
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

const notificationsQueue = []

export default function () {
    const page = /** @type {DocumentFragment} */ (template.content.cloneNode(true))
    const notificationsLink = document.querySelector('#notifications-link.unread')
    const flushQueueButton = page.getElementById('flush-queue')
    const notificationsDiv = page.getElementById('notifications')

    if (notificationsLink !== null) {
        notificationsLink.classList.remove('unread')
    }

    flushQueueButton.addEventListener('click', () => {
        let notification
        while (notification = notificationsQueue.shift()) {
            notificationsDiv.insertBefore(
                createNotificationLink(notification),
                notificationsDiv.firstChild)
        }
        flushQueueButton.hidden = true
    })

    http.get('/api/notifications').then(notifications => {
        notifications.forEach(notification => {
            notificationsDiv.appendChild(createNotificationLink(notification))
        })
    }).catch(console.error)

    /**
     * @param {CustomEvent} ev
     */
    const onNotification = ev => {
        const { detail: notification } = ev
        notificationsQueue.push(notification)
        const l = notificationsQueue.length
        flushQueueButton.textContent = `${l} new notification${l !== 1 ? 's' : ''}`
        flushQueueButton.hidden = false
    }

    addEventListener('notification', onNotification)
    page.addEventListener('disconnect', () => {
        removeEventListener('notification', onNotification)
    })

    return page
}
