import http from '../http.js'
import html from '../html.js'

const template = html`
<div class="container">
    <h1>Welcome to Nakama 👋</h1>
    <form id="login">
        <input type="email" placeholder="Email" value="john@example.dev" autocomplete="email" autofocus required>
        <button type="submit">Login</button>
    </form>
</div>
`

export default function () {
    const page = /** @type {DocumentFragment} */ (template.content.cloneNode(true))
    const loginForm = /** @type {HTMLFormElement} */ (page.getElementById('login'))
    const loginInput = loginForm.querySelector('input')
    const loginButton = loginForm.querySelector('button')

    loginForm.addEventListener('submit', ev => {
        ev.preventDefault()
        const email = loginInput.value.trim()

        if (email === '') {
            loginInput.setCustomValidity('Empty')
            return
        }

        loginInput.disabled = true
        loginButton.disabled = true

        http.post('/api/login', { email }).then(payload => {
            localStorage.setItem('expires_at', payload.expiresAt)
            localStorage.setItem('auth_user', JSON.stringify(payload.user))
            location.reload()
        }).catch(err => {
            console.error(err)
            if ('email' in err) {
                loginInput.setCustomValidity(err['email'])
            } else {
                alert(err.message)
            }
            loginInput.disabled = false
            loginButton.disabled = false
            loginInput.focus()
        })
    })

    loginInput.addEventListener('input', () => {
        loginInput.setCustomValidity('')
    })

    return page
}
