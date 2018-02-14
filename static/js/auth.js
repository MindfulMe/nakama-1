function isExpired() {
    const expiresAtItem = localStorage.getItem('expires_at')
    if (expiresAtItem === null) return true
    const expiresAt = new Date(expiresAtItem)
    return isNaN(expiresAt.getDate()) || expiresAt <= new Date()
}

/**
 * @returns {AuthUser=}
 */
export function getAuthUser() {
    if (isExpired()) return null

    const authUserItem = localStorage.getItem('auth_user')
    if (authUserItem === null) return null
    try {
        return JSON.parse(authUserItem)
    } catch (_) { }
    return null
}

/**
 * @typedef AuthUser
 * @property {string} username
 * @property {string=} avatarUrl
 */
