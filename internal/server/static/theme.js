// static/theme.js
document.addEventListener('DOMContentLoaded', () => {
  // Переключатель темы
  const toggle = document.getElementById('theme-switch');
  const body   = document.body;
  const root   = document.documentElement;

  // временно убираем анимацию, чтобы кружок не «скакал» при первой отрисовке
  root.classList.add('no-anim');

  // начальное состояние: localStorage → иначе системная настройка
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  if (localStorage.theme === 'dark' || (!localStorage.theme && prefersDark)) {
    body.classList.add('theme-dark');   // <body>
  }
  if (toggle) toggle.checked = body.classList.contains('theme-dark');

  // возвращаем анимацию – теперь она будет работать ТОЛЬКО по кликам
  setTimeout(() => root.classList.remove('no-anim'), 0);

  // переключатель
  if (toggle) toggle.addEventListener('change', () => {
    const dark = toggle.checked;
    body.classList.toggle('theme-dark', dark);
    root.classList.toggle('theme-dark', dark);
    localStorage.theme = dark ? 'dark' : 'light';
  });
});
