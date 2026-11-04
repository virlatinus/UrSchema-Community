import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'

// Login state and server mode, from GET /api/auth/status
// (see backend/handler/rpc/server.go).
export const useSessionStore = defineStore('session', () => {
  const loaded = ref(false)
  const authRequired = ref(false)
  const authenticated = ref(false)
  const readOnly = ref(false)

  async function fetchStatus() {
    try {
      const { data } = await axios.get('api/auth/status')
      authRequired.value = !!data.authRequired
      authenticated.value = !!data.authenticated
      readOnly.value = !!data.readOnly
    } finally {
      // Render the app even if the backend is unreachable; its own
      // error handling reports that.
      loaded.value = true
    }
  }

  async function login(password) {
    await axios.post('api/auth/login', { password })
    authenticated.value = true
  }

  async function logout() {
    await axios.post('api/auth/logout')
    authenticated.value = false
  }

  // Any 401 from the API means the session expired or the server restarted.
  axios.interceptors.response.use(undefined, (error) => {
    if (error?.response?.status === 401 && !error.config?.url?.endsWith('/auth/login')) {
      authenticated.value = false
    }
    return Promise.reject(error)
  })

  return { loaded, authRequired, authenticated, readOnly, fetchStatus, login, logout }
})
