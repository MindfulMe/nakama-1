/**
 * @param {TemplateStringsArray} strings
 * @param {...any} values
 */
export default function html(strings, ...values) {
    const rawStrings = strings.raw
    const template = document.createElement('template')
    template.innerHTML = values.reduce((acc, v, i) =>
        acc + String(v) + rawStrings[i + 1], rawStrings[0])
    return template
}
