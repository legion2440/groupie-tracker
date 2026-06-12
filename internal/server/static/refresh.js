document.addEventListener('DOMContentLoaded', () => {
  // Кнопка обновления информации
  const btn = document.getElementById('refresh-btn');
  if (!btn) return;

  // Всплывашка
  function toast(msg, isErr=false){ 
    console.log(isErr ? 'ERR:' : 'OK:', msg); 
  }

  // Обработчик клика по кнопке "Обновить информацию"
  btn.addEventListener('click', async () => {
    btn.disabled = true;
    btn.classList.remove('error');
    btn.textContent = 'Обновляю…';

    let ok = false;
    try {
      // Отправляем запрос на обновление кеша
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

    // финальный шаг — включаем кнопку обратно
    btn.disabled = false;

    // красная подсветка держится, пока пользователь не нажмёт снова
    if (!ok) btn.classList.add('error');
    else {
      // после успешного обновления через 3 с вернём исходный текст
      setTimeout(() => { btn.textContent = 'Обновить информацию'; }, 3000);
    }
  });

  // онлайн-/оффлайн-индикатор
  window.addEventListener('offline', () => { btn.disabled = true; });
  window.addEventListener('online',  ()  => { btn.disabled = false; });
});
