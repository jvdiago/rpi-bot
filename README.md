# rpi-bot

A Go-based bot designed to execute commands on a Raspberry Pi (or any Linux system) triggered by messages from Telegram, Signal, or HTTP requests.

## Features

*   **Multi-Platform Support:** Responds to commands from Telegram, Signal, or HTTP requests.
*   **Command Configuration:**  Define commands and their arguments in a YAML configuration file.
*   **Command Execution:** Executes commands on the host operating system.

## Prerequisites

*   Go (1.18 or later)
*   A Raspberry Pi (or any Linux system)
*   A Telegram bot API token (if using Telegram)
*   `signal-cli` installed and configured (if using Signal)

## Installation

1.  **Clone the repository:**

    ```bash
    git clone https://github.com/jvdiago/rpi-bot.git
    cd rpi-bot
    ```

2.  **Build the application:**

    ```bash
    go build -o rpi-bot
    ```

## Configuration

The application is configured via a YAML file (default: `config.yaml`). Here's an example configuration:

```yaml
commands:
  hostname:
    command: hostname
    args: []
  df:
    command: df -h %s
    args: ["path"]
  free:
    command: free -m
    args: []
  reboot:
    command: sudo reboot
    args: []
signal:
  sources:
  - "+15551234567"
  socket: /run/user/1000/signal-cli.socket
telegram:
  debug: true
provider: telegram # or signal or "" for disabled
httpd:
  enabled: true
  addr: ":8080"
  authToken: "YOUR_HTTP_AUTH_TOKEN"
```

### Configuration Options

*   **`commands`:** A map of command names to their definitions.
    *   **`command`:** The command to execute.  Use `%s` as placeholders for arguments.
    *   **`args`:** A list of argument names.  These names are used when constructing HTTP requests.

*   **`signal`:** Configuration for Signal integration.
    *   **`sources`:** A list of Signal phone numbers that the bot will respond to.
    *   **`socket`:** The path to the `signal-cli` socket.  This is usually `/run/user/<uid>/signal-cli.socket`, replace `<uid>` with the user id running signal-cli.

*   **`telegram`:** Configuration for Telegram integration.
    *   **`debug`:** Enables debug logging.
    *   **`apiToken`:** The Telegram bot API token. You can also set this using the `TELEGRAM_APITOKEN` environment variable, which will override this setting.

*   **`provider`:** Specifies the messaging provider to use.  Valid values are `"telegram"`, `"signal"`. Set to empty string to disable.

*   **`httpd`:** Configuration for the HTTP server.
    *   **`enabled`:** Enables the HTTP server.
    *   **`addr`:** The address to listen on (e.g., `":8080"`).
    *   **`authToken`:** An authentication token for HTTP requests.  You can also set this using the `HTTP_TOKEN_AUTH` environment variable, which will override this setting.

## Usage

1.  **Create a `config.yaml` file** based on the example above, adjusting the values to your environment.

2.  **Set environment variables (optional):**

    *   `TELEGRAM_APITOKEN`: Your Telegram bot API token.
    *   `HTTP_TOKEN_AUTH`:  Your HTTP authentication token.

3.  **Run the application:**

    ```bash
    ./rpi-bot -config config.yaml
    ```

    Or without specifying the configuration file, the application will use the default `./config.yaml` file.

    ```bash
    ./rpi-bot
    ```

## Messaging Systems

### Telegram

1.  **Create a Telegram bot** using BotFather.
2.  **Obtain the bot API token.**
3.  **Configure the `telegram` section** in `config.yaml` with the API token.
4.  **Set the `provider`** to `"telegram"` in `config.yaml`.
5.  **Send commands to the bot** using the `/command` syntax (e.g., `/hostname`).

### Signal

1.  **Install and configure `signal-cli`** on your system.  Make sure the `signal-cli` daemon is running.
```
signal-cli -u +15551234567 daemon --socket /run/user/1000/signal-cli.socket
```
2.  **Configure the `signal` section** in `config.yaml` with the socket path and allowed source phone numbers.
3.  **Set the `provider`** to `"signal"` in `config.yaml`.
4.  **Send commands to the bot** by sending a message starting with `/` (e.g., `/hostname`).

### HTTPD

1.  **Configure the `httpd` section** in `config.yaml`, setting `enabled` to `true`, the `addr`, and an `authToken`.
2.  **Send HTTP requests** to the `/cmd/<command>` endpoint with the `Authorization` header set to `Token <authToken>` and any required arguments as query parameters. If no authToken is configured the httpd endpoints will be unauthenticated

    Example:

    ```bash
    curl -H "Authorization: Token YOUR_HTTP_AUTH_TOKEN" "http://localhost:8080/cmd/status"
    ```

## Examples

### Configuration:

```yaml
commands:
  hostname:
    command: hostname
    args: []
  df:
    command: df -h %s
    args: ["path"]
  free:
    command: free -m
    args: []
  reboot:
    command: sudo reboot
    args: []
```

### Telegram:

Send `/hostname` to the bot. The bot will reply with the hostname of the Raspberry Pi.

### Signal:

Send `/df /` to the bot. The bot will reply with the disk space usage of the root directory.

### HTTPD:

```bash
curl -H "Authorization: Token YOUR_HTTP_AUTH_TOKEN" "http://localhost:8080/cmd/free"
```

The server will respond with the output of the `free -m` command.

```bash
curl -H "Authorization: Token YOUR_HTTP_AUTH_TOKEN" "http://localhost:8080/cmd/df?path=/"
```

The server will respond with the output of the `df -h /` command.

## Security Considerations

*   **Protect your Telegram bot API token.** Do not commit it to your repository or share it publicly.  Use environment variables or secure configuration management practices.
*   **Use a strong authentication token** for the HTTP server.
*   **Be careful about the commands you expose.** Avoid commands that could be used to compromise your system. Consider limiting the commands to a safe subset.
*   **For signal-cli, ensure the socket file has appropriate permissions** to prevent unauthorized access.

## Contributing

Contributions are welcome! Please submit a pull request.

## License

[MIT License](LICENSE)
