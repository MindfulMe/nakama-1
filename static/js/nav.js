import { authenticated, authUser } from './auth.js'
import http from './http.js'

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
