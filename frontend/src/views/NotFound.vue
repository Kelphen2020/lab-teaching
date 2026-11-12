<template>
  <div class="error-page">
    <div class="error-code">404</div>
    <div class="error-title">页面不存在</div>
    <div class="error-desc">您访问的地址可能已被移除、拼写错误或暂时不可用</div>
    <div class="error-actions">
      <router-link to="/" class="btn btn-primary"><SvgIcon name="home" /> 返回首页</router-link>
      <button class="btn" @click="$router.back()">↩ 返回上一页</button>
    </div>
  </div>
</template>

<style scoped>
.error-page { display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 60vh; text-align: center; }
.error-code { font-size: 96px; font-weight: 800; color: var(--muted); line-height: 1; }
.error-title { font-size: 22px; font-weight: 700; color: var(--text); margin: 12px 0 8px; }
.error-desc { font-size: 14px; color: var(--muted); margin-bottom: 24px; max-width: 360px; }
.error-actions { display: flex; gap: 12px; }
</style>