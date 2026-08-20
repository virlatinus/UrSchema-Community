/**
 * Toast/dialog helper built on naive-ui's message + dialog providers
 * (mounted at the app root in App.vue).
 */
import { useMessage, useDialog } from 'naive-ui'

export function useNotify() {
    const message = useMessage()
    const dialog = useDialog()

    return {
        success(msg, options) {
            message.success(msg, options)
        },

        error(msg, options) {
            message.error(msg, options)
        },

        info(msg, options) {
            message.info(msg, options)
        },

        warning(msg, options) {
            message.warning(msg, options)
        },

        /**
         * Show a confirm dialog. Resolves true/false.
         */
        async confirm(options = {}) {
            return new Promise((resolve) => {
                dialog.warning({
                    title: options.title || 'Confirm',
                    content: options.content || '',
                    positiveText: options.positiveText || 'OK',
                    negativeText: options.negativeText || 'Cancel',
                    maskClosable: false,
                    onPositiveClick: () => resolve(true),
                    onNegativeClick: () => resolve(false),
                    onClose: () => resolve(false)
                })
            })
        },

        /**
         * Show an alert dialog. Resolves once dismissed.
         */
        async alert(options = {}) {
            return new Promise((resolve) => {
                dialog.info({
                    title: options.title || 'Notice',
                    content: options.content || '',
                    positiveText: options.positiveText || 'OK',
                    onPositiveClick: () => resolve(true),
                    onClose: () => resolve(true)
                })
            })
        },

        async copyToClipboard(text) {
            const value = String(text || '')
            if (!value) return false

            if (navigator.clipboard?.writeText) {
                await navigator.clipboard.writeText(value)
                return true
            }

            return false
        }
    }
}
