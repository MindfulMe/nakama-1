import http from '../http.js'

export default function () {
    const fragment = new URLSearchParams(decodeURIComponent(location.hash.substr(1)))
    const expiresAt = fragment.get('expires_at')

    if (typeof expiresAt === 'string' && !isNaN(new Date(expiresAt).getDate())) {
        localStorage.setItem('expires_at', expiresAt)
        http.get('/api/me').then(authUser => {
            localStorage.setItem('auth_user', JSON.stringify(authUser))
        }).catch(console.error).then(() => {
            location.replace('/')
        })
    } else {
        location.replace('/')
    }

    return document.createDocumentFragment()
}
