/**
 * Connection Configuration API Client
 * Communicates with backend to manage saved connections
 */

import axios from 'axios'

class ConnectionConfigAPI {
    constructor() {
        // Relative, so the app also works under a URL prefix (URSCHEMA_BASE_PATH)
        this.baseURL = 'api'
    }

    /**
     * Make a request to backend
     */
    async request(method, params = {}) {
        const response = await axios.post(`${this.baseURL}/${method}`, { params })
        const rpcResponse = response.data

        if (!rpcResponse || rpcResponse.code !== 200) {
            throw new Error(rpcResponse?.msg || 'RPC request failed')
        }

        return rpcResponse.data
    }

    /**
     * Save connection configuration
     */
    async saveConnection(config) {
        return this.request('db.saveConnectionConfig', config)
    }

    /**
     * List all saved connections (without passwords)
     */
    async listConnections() {
        return this.request('db.listConnectionConfigs')
    }

    /**
     * Get single connection (with password)
     */
    async getConnection(id) {
        return this.request('db.getConnectionConfig', { id })
    }

    /**
     * Delete connection
     */
    async deleteConnection(id) {
        return this.request('db.deleteConnectionConfig', { id })
    }
}

export default new ConnectionConfigAPI()
