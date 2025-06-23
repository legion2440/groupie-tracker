document.addEventListener('DOMContentLoaded', () => {
  const btn = document.getElementById('refresh-btn');
  if (!btn) return;

  btn.addEventListener('click', async () => {
    btn.disabled = true;
    btn.textContent = 'Обновляю...';

    try {
      const res = await fetch('/api/refresh', { method: 'POST' });
      if (res.status === 202) {
        btn.textContent = 'Кеш обновлён ✓';
      } else {
        btn.textContent = 'Ошибка (' + res.status + ')';
      }
    } catch (e) {
      btn.textContent = 'Сеть недоступна';
    }
    setTimeout(() => {
      btn.disabled = false;
      btn.textContent = 'Обновить концерты';
    }, 3000);
  });
});
