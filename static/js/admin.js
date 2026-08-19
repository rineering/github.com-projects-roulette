function triggerSpin() {
    // Тут будет наш микро-скрипт отправки команды на Go сервер
    console.log("Крутим!");
}
function toggleTokenVisibility() {
    const input = document.getElementById('daTokenInput');
    if (input.type === "password") {
        input.type = "text";
    } else {input.type = "password";

    }
}

function showToast(message, status) {
    const toastBody = document.getElementById('toastBody');
    const toastElement = document.getElementById('liveToast');
    
    if (!toastBody || !toastElement) {
        console.error("Элементы Toast не найдены в HTML!");
        return;
    }

    // Меняем текст уведомления
    toastBody.innerText = message;

    // Меняем цвет плашки в зависимости от статуса (success/error)
    if (status === 200) {
        toastElement.classList.remove('bg-danger');
        toastElement.classList.add('bg-success');
    } else {
        toastElement.classList.remove('bg-success');
        toastElement.classList.add('bg-danger');
    }

    // Запускаем анимацию всплытия через Bootstrap JS
    const toast = new bootstrap.Toast(toastElement);
    toast.show();
}


// AJAX отправка формы настроек без перезагрузки страницы
document.getElementById('configForm').addEventListener('submit', function(e) {
    e.preventDefault();

    const formData = new FormData(this);
    fetch(this.action, {
        method: 'POST',
        body: new URLSearchParams(formData) 
        // отправляем как application/x-www-form-urlencoded
    }).then(response => {
        //console.log(response);
        if(response.ok) {
            showToast("⚙️ Настройки успешно сохранены!", response.status);
        } else {
            showToast("❌ Ошибка при сохранении настроек", response.status);
        }
    }).catch(err => {
        showToast("❌ Сетевая ошибка", false);
        console.error(err);
    });
});

// Обработчик кнопки ручного прокрута (AJAX запрос к Go)
function triggerSpin() {
    const btn = document.getElementById('spinBtn');
    btn.disabled = true;
    fetch('/api/trigger-spin', { method: 'POST' }) 
    // Предположим, что у вас такой эндпоинт на Go
    .then(response => {
        //console.log(response);
        if(response.ok) {
            showToast("🎰 Команда на прокрутку отправлена!");
        } else {
            showToast("❌ Не удалось запустить рулетку", response.status);
        }
    })
    .catch(err => showToast("❌ Ошибка соединения", response.status))
    .finally(() => {
        setTimeout(() => { btn.disabled = false; }, 2000); // Разблокируем через 2 сек
    });
}