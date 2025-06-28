// static/refresh.js
document.addEventListener('DOMContentLoaded', () => {
  const btn = document.getElementById('refresh-btn');
  if (!btn) return;

  /* всплывашка, оставьте вашу реализацию */
  function toast(msg, isErr=false){ console.log(isErr ? 'ERR:' : 'OK:', msg); }

  btn.addEventListener('click', async () => {
    btn.disabled = true;
    btn.classList.remove('error');
    btn.textContent = 'Обновляю…';

    let ok = false;
    try {
      const res = await fetch('/api/refresh', { method:'POST' });
      if (res.ok){
        ok = true;
        toast('Кеш обновлён');
        btn.textContent = 'Кеш обновлён ✓';
      } else {
        toast('Ошибка ' + res.status, true);
        btn.textContent = 'Ошибка обновления';
      }
    } catch {
      toast('Нет соединения', true);
      btn.textContent = 'Сеть недоступна';
    }

    /* финальный шаг — включаем кнопку обратно */
    btn.disabled = false;

    /* красная подсветка держится, пока пользователь не нажмёт снова */
    if (!ok) btn.classList.add('error');
    else {
      /* после успешного обновления через 3 с вернём исходный текст */
      setTimeout(() => { btn.textContent = 'Обновить информацию'; }, 3000);
    }
  });

  /* онлайн-/оффлайн-индикатор */
  window.addEventListener('offline', () => { btn.disabled = true; });
  window.addEventListener('online',  ()  => { btn.disabled = false; });


  /* ── dark / light toggle ─────────────────── */
  const toggle = document.getElementById('theme-switch');
  const body   = document.body;
  
  /* временно убираем анимацию, чтобы кружок не «скакал» при первой отрисовке */
  const root = document.documentElement;      // <html>
  root.classList.add('no-anim');

  // начальное состояние: localStorage → иначе системная настройка
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  if (localStorage.theme === 'dark' || (!localStorage.theme && prefersDark)) {
    body.classList.add('theme-dark');   // <body>
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
