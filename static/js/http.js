import { isObject } from './utils.js'

/**
 * Parses an HTTP response.
 *
 * @param {Response} res
 */
export async function handleResponse(res) {
    const ct = res.headers.get('Content-Type')
    const isJSON = ct !== null && ct.startsWith('application/json')

    const payload = await res[isJSON ? 'json' : 'text']()

    if (!res.ok) {
        const err = new Error(res.statusText)
        err['statusCode'] = res.status
        if (isObject(payload)) Object.assign(err, payload)
        else if (typeof payload === 'string' && payload !== '') err.message = payload
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
 * @param {{string: string}=} headers
 */
function post(url, payload, headers) {
    const options = {
        method: 'POST',
        credentials: 'include',
        headers: {},
    }
    if (isObject(payload)) {
        options['body'] = JSON.stringify(payload)
        options.headers['Content-Type'] = 'application/json'
    } else if (payload !== undefined) {
        options['body'] = payload
    }
    Object.assign(options.headers, headers)
    // @ts-ignore
    return fetch(url, options).then(handleResponse)
}

export default {
    handleResponse,
    get,
    post,
}
