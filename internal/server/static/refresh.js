// static/refresh.js
document.addEventListener('DOMContentLoaded', () => {
  /* ── refresh ─────────────────────────────── */
  const btn = document.getElementById('refresh-btn');
  if (btn) {
    btn.addEventListener('click', async () => {
      btn.disabled = true;
      btn.textContent = 'Обновляю...';
      try {
        const res = await fetch('/api/refresh', { method: 'POST' });
        btn.textContent = res.status === 202 ? 'Кеш обновлён ✓' : 'Ошибка';
      } catch { btn.textContent = 'Сеть недоступна'; }
      setTimeout(() => { btn.disabled = false; btn.textContent = 'Обновить информацию'; }, 3000);
    });
  }

  /* ── dark / light toggle ─────────────────── */
  const toggle = document.getElementById('theme-switch');
  const body   = document.body;
  
  /* временно убираем анимацию, чтобы кружок не «скакал» при первой отрисовке */
  const root = document.documentElement;      // <html>
  root.classList.add('no-anim');

  // начальное состояние: localStorage → иначе системная настройка
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  if (localStorage.theme === 'dark' || (!localStorage.theme && prefersDark)) {
    body.classList.add('theme-dark');
    root.classList.add('theme-dark');
  }
  if (toggle) toggle.checked = body.classList.contains('theme-dark');

  /* возвращаем анимацию – теперь она будет работать ТОЛЬКО по кликам */
  setTimeout(() => root.classList.remove('no-anim'), 0);

  // переключатель
  if (toggle) toggle.addEventListener('change', () => {
    const dark = toggle.checked;
    body.classList.toggle('theme-dark', dark);
    root.classList.toggle('theme-dark', dark);
    localStorage.theme = dark ? 'dark' : 'light';
  });
});
