## Consumir Mensajes en Frontend

* Endpoint //notify-check-coins: Este endpoint devuelve los mensajes almacenados en MessageStore. Desde el frontend, se puede realizar una solicitud HTTP GET a este endpoint para obtener todos los mensajes publicados y mostrarlos en la interfaz de usuario.

* Endpoint /notify-open-box: Este endpoint devuelve las 10 monedas más rankeadas. Al igual que con /messages, el frontend puede hacer una solicitud HTTP GET a este endpoint para obtener la lista de monedas y presentarlas a los usuarios.

* Importante: Ya esta subido a gcp y probado de que funcione los endpoints, el /messages lo probe cada 1 minuto para ver que funcionaba, al final lo deje para que lo muestre cada 8hrs como se requeria y el /top-coins muestra las 10 monedas mas rankeadas de la api binance y esta funcionando.
