<?php

declare(strict_types=1);

// Punto de entrada para FrankenPHP, que sirve un directorio publico. La logica
// vive en src/server.php: __DIR__ dentro de ese archivo sigue apuntando a src/,
// asi que su require del autoload resuelve igual.
require __DIR__ . '/../src/server.php';
