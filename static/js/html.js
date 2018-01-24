const htmlValue = value => value instanceof Element
    ? value.innerHTML
    : String(value)

/**
 * @param {TemplateStringsArray} strings
 * @param {...any} values
 */
export default function html(strings, ...values) {
    const rawStrings = strings.raw
    const template = document.createElement('template')
    template.innerHTML = values.reduce((acc, v, i) =>
        acc + htmlValue(v) + rawStrings[i + 1], rawStrings[0])
    return template
}
