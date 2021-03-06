import { isObject } from './utils.js'

/**
 * Parses an HTTP response.
 *
 * @param {Response} res
 */
export async function handleResponse(res) {
    if (res.status === 401) {
        localStorage.removeItem('jwt')
        localStorage.removeItem('auth_user')
        localStorage.removeItem('expires_at')
    }

    const ct = res.headers.get('content-type')
    const isJSON = typeof ct === 'string' && ct.startsWith('application/json')

    let payload = await res[isJSON ? 'json' : 'text']()
    if (!isJSON) {
        try {
            payload = JSON.parse(payload)
        } catch (_) {
            payload = { message: payload }
        }
    }

    if (!res.ok) {
        const err = new Error(res.statusText)
        err['statusCode'] = res.status
        Object.assign(err, payload)
        throw err
    }

    return payload
}

/**
 * Does a GET request.
 *
 * @param {string} url
 */
const get = url => fetch(url, { credentials: 'include' }).then(handleResponse)

/**
 * Does a POST request.
 *
 * @param {string} url
 * @param {any=} payload
 * @param {{ [key: string]: string }=} headers
 */
function post(url, payload, headers) {
    const options = {
        method: 'POST',
        credentials: 'include',
        headers: {},
    }
    if (payload instanceof File) {
        options['body'] = payload
    } else if (isObject(payload)) {
        options['body'] = JSON.stringify(payload)
        options.headers['Content-Type'] = 'application/json'
    } else if (payload !== undefined && payload !== null) {
        options['body'] = payload
    }
    Object.assign(options.headers, headers)
    // @ts-ignore
    return fetch(url, options).then(handleResponse)
}

/**
 * Creates a Server-Sent Event connection
 *
 * @param {string} url
 * @param {function} callback
 */
function subscribe(url, callback) {
    // @ts-ignore
    const eventSource = new EventSource(url)
    eventSource.onmessage = ev => {
        try {
            const payload = JSON.parse(ev.data)
            callback(payload)
        } catch (_) { }
    }
    return () => {
        eventSource.close()
    }
}

export default {
    handleResponse,
    get,
    post,
    subscribe,
}
