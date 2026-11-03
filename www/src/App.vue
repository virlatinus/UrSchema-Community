<template>
  <n-config-provider
    :theme="darkTheme"
    :theme-overrides="themeOverrides"
    :locale="enUS"
    :date-locale="dateEnUS"
  >
    <n-message-provider>
      <n-dialog-provider>
        <n-notification-provider>
          <div id="app-window">
            <template v-if="session.loaded">
              <LoginView v-if="session.authRequired && !session.authenticated" />
              <MainLayout v-else />
            </template>
          </div>
        </n-notification-provider>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup>
import { onMounted } from 'vue'
import { darkTheme, enUS, dateEnUS } from 'naive-ui'
import MainLayout from '@/views/MainLayout.vue'
import LoginView from '@/views/LoginView.vue'
import { themeOverrides } from '@/styles/naive-theme.js'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
onMounted(() => session.fetchStatus())
</script>

<style>
.form-modal {
  background: transparent;
}

.form-modal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--ref-color-white-8);
  font-size: 16px;
  font-weight: 500;
}

.form-modal-body {
  padding: 20px;
}
</style>
