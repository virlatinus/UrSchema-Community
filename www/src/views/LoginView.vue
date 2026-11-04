<template>
  <main class="login-screen">
    <form class="login-card" @submit.prevent="submit">
      <img src="/icon.png" width="48" height="48" alt="" />
      <h1>{{ t('login.title') }}</h1>
      <n-input
        v-model:value="password"
        type="password"
        show-password-on="click"
        :placeholder="t('login.password')"
        :input-props="{ autocomplete: 'current-password', autofocus: true }"
      />
      <p v-if="error" class="login-error">{{ error }}</p>
      <n-button type="primary" attr-type="submit" block :loading="loading" :disabled="!password">
        {{ t('login.submit') }}
      </n-button>
    </form>
  </main>
</template>

<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSessionStore } from '@/stores/session'

const { t } = useI18n()
const session = useSessionStore()
const password = ref('')
const loading = ref(false)
const error = ref('')

async function submit() {
  loading.value = true
  error.value = ''
  try {
    await session.login(password.value)
  } catch (e) {
    const status = e?.response?.status
    if (status === 429) {
      error.value = t('login.tooMany', { s: e.response.headers['retry-after'] || '?' })
    } else {
      error.value = status === 401 ? t('login.invalid') : t('login.failed', { msg: e.message })
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-screen {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
}

.login-card {
  width: 100%;
  max-width: 320px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--ref-space-12);
  padding: 32px 24px;
  border: 1px solid var(--ref-color-white-8);
  border-radius: var(--ref-radius-xl);
  background: var(--ref-color-white-4);
}

.login-card h1 {
  font-size: 16px;
  font-weight: 500;
  color: var(--ref-color-white-85);
  margin-bottom: var(--ref-space-4);
}

.login-error {
  align-self: flex-start;
  font-size: 12px;
  color: #ff4d4f;
}
</style>
