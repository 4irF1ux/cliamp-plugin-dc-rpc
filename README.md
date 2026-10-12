# dc-rpc

Muestra lo que estás escuchando en [cliamp](https://cliamp.stream) como Discord Rich Presence.

Son dos piezas:

- **Plugin Lua** `dc-rpc.lua`: publica el estado de reproducción por el socket IPC de cliamp.
- **Daemon** `cliamp-rpcd`: se conecta a Discord y actualiza la tarjeta. Se cierra solo si cliamp deja de responder.

## Instalación

Requisitos: cliamp con plugins, Discord abierto en la misma máquina y Go 1.22 o superior para compilar el daemon. Todavía no hay release con binarios.

1. Instalá el plugin y confiá en él:

   ```sh
   cliamp plugins install 4irF1ux/cliamp-plugin-dc-rpc
   ```

2. Compilá el daemon y ponelo en una carpeta que esté en el PATH:

   ```sh
   git clone https://github.com/4irF1ux/cliamp-plugin-dc-rpc
   cd cliamp-plugin-dc-rpc
   go build -o cliamp-rpcd ./cmd/cliamp-rpcd
   ```

   En Windows, el binario se llama `cliamp-rpcd.exe`.

3. Reiniciá cliamp.

Hoy el daemon se arranca a mano (`cliamp-rpcd`). El plugin todavía no lo lanza.

## Configuración

```toml
[plugins.dc-rpc]
enabled = true     # false apaga el plugin y quita la tarjeta
show_stream = true # false oculta título y artista en streams (radios, etc.)
```

`enabled` y `show_stream` se leen al cargar el plugin. Después de cambiarlos, reiniciá cliamp.

## Desinstalación

```sh
cliamp plugins remove dc-rpc
```

Esto quita el plugin. El binario `cliamp-rpcd` queda; borralo a mano si no lo usás.

## Cómo funciona el daemon

- Se conecta al socket de cliamp y se suscribe al estado de reproducción.
- Si cliamp deja de responder durante más de un minuto, el daemon se cierra solo. Esto cubre el caso de un cierre forzado de cliamp.
- Si Discord se reinicia, el daemon reintenta cada 5 s.
- La portada es la URL pública `https` del track. Sin portada, la tarjeta usa el asset `cliamp` de la app de Discord del proyecto.

## Limitaciones

- Solo Windows está probado. Linux y macOS no.
- Las portadas embebidas en archivos locales no se muestran; solo URLs `https` públicas.
- El client ID de Discord es fijo en el código (`defaultClientID` en `cmd/cliamp-rpcd/main.go`). Si usás tu propia app de Discord, cambialo, recompilá y subí un asset llamado `cliamp` en esa app.
- Discord cachea la imagen por defecto: un cambio de asset puede tardar en verse.
