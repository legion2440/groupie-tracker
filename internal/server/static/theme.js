// static/theme.js
document.addEventListener('DOMContentLoaded', () => {
  // Переключатель темы
  const toggle = document.getElementById('theme-switch');
  const body   = document.body;
  const root   = document.documentElement;

  // временно убираем анимацию при первой отрисовке
  root.classList.add('no-anim');

  // начальное состояние: localStorage → иначе системная настройка
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  const initialDark = localStorage.theme === 'dark' || (!localStorage.theme && prefersDark);

  body.classList.toggle('theme-dark', initialDark);
  root.classList.toggle('theme-dark', initialDark);
  if (toggle) toggle.setAttribute('aria-pressed', String(initialDark));

  // возвращаем анимацию - работает только по кликам
  setTimeout(() => root.classList.remove('no-anim'), 0);

  // переключатель
  if (toggle) toggle.addEventListener('click', () => {
    const dark = !body.classList.contains('theme-dark');

    body.classList.toggle('theme-dark', dark);
    root.classList.toggle('theme-dark', dark);
    localStorage.theme = dark ? 'dark' : 'light';
    toggle.setAttribute('aria-pressed', String(dark));
  });
});
