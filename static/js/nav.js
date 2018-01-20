import { authenticated, authUser } from './auth.js'
import http from './http.js'
import { getNotificationMessage, getNotificationHref, goto } from './utils.js';

const nav = document.getElementById('nav')
nav.className = 'app-nav'
nav.innerHTML = `
    <a href="/">Home</a>
    ${authenticated ? `
        <a href="/notifications" id="notifications-link">Notifications</a>
    ` : ''}
    <a href="/search">Search</a>
    ${authenticated ? `
        <a href="/users/${authUser.username}">Profile</a>
    ` : ''}
`

const notificationLink = nav.querySelector('#notifications-link')

if (authenticated && location.pathname !== '/notifications') {
    http.get('/api/check_unread_notifications').then(unread => {
        if (unread) {
            notificationLink.classList.add('unread')
        }
    }).catch(console.error)
}

if (authenticated) {
    http.subscribe('/api/notifications', notification => {
        const { pathname } = location
        if (pathname === '/notifications') {
            dispatchEvent(new CustomEvent('notification', { detail: notification }))
            return
        }
        const match = /^\/posts\/([^\/]+)$/.exec(pathname)
        if (match !== null
            && /** @type {string} */ (notification.verb).startsWith('comment')
            && notification.targetId === match[1]) {
            return
        }
        notificationLink.classList.add('unread')
        Notification.requestPermission().then(permission => {
            if (permission !== 'granted') return
            const message = getNotificationMessage(notification)
            if (message === null) return
            const n = new Notification('New Notification', {
                body: message,
                tag: notification.id,
            })
            n.onclick = () => {
                goto(getNotificationHref(notification))
                n.close()
            }
        })
    })
}
