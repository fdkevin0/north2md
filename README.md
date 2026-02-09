# south2md

`south2md` is a command-line tool written in Go to scrape posts from the South Plus forum and convert them into clean, readable Markdown files. It can fetch posts directly by their thread ID, parse local HTML files, and download all associated images and attachments.

## Features

- **Fetch Online Posts**: Scrape forum posts directly by providing a thread ID (TID).
- **Parse Local Files**: Convert locally saved HTML files into Markdown.
- **Attachment Downloading**: Automatically download and cache images and other attachments from the post.
- **Gofile 同步**: 识别并下载 `gofile.io` 分享链接到 `tid/gofile/`，并在 Markdown 中同时保留原始链接与本地相对路径。
- **Cookie-Based Authentication**: Use a standard Netscape cookie file to access restricted or members-only content.
- **Markdown Formatting**: Generate well-formatted Markdown with options to include author information, table of contents, and more.
- **Configurable**: Customize the tool's behavior through command-line flags or a TOML configuration file.

## Installation

To install `south2md`, you need to have Go installed on your system. You can then build and install the tool using the following command:

```sh
go install github.com/fdkevin0/south2md@latest
```

## Usage

### Fetching an Online Post

To fetch a post, simply provide the thread ID (TID) as an argument:

```sh
south2md <TID>
```

For example:

```sh
south2md 2636739 --output=post.md
```

### Parsing a Local HTML File

If you have a post saved as an HTML file, you can parse it using the `--input` flag:

```sh
south2md --input=post.html --output=post.md
```

### Using Cookies for Authentication

To access restricted content, you can use a standard Netscape cookie file.

1.  **Import Cookies**:
    Provide a standard cookie file and cache it into your user data space (XDG data home).

    ```sh
    south2md cookie import --file=./cookies.txt
    ```

    This will cache the file to `$XDG_DATA_HOME/south2md/cookies.txt` (or `~/.local/share/south2md/cookies.txt`).

2.  **Fetch with Cookies**:
    Now, you can use the `--cookie-file` flag to fetch the post:

    ```sh
    south2md 2636739 --cookie-file=./cookies.txt --output=post.md
    ```

### Command-Line Flags

Here are all the available command-line flags:

| Flag              | Description                                     | Default                |
| ----------------- | ----------------------------------------------- | ---------------------- |
| `--config`        | TOML config file path                           | auto-discover          |
| `--tid`           | Thread ID (for online fetching)                 |                        |
| `--input`         | Input HTML file path                            |                        |
| `--output`        | Output Markdown file path                       | `post.md`              |
| `--cache-dir`     | Directory for caching attachments               | `~/.cache/south2md`    |
| `--base-url`      | Base URL of the forum                           | `https://south-plus.net/` |
| `--cookie-file`   | Path to the cookie file (Netscape format)       | `~/.local/share/south2md/cookies.txt` |
| `--no-cache`      | Disable attachment caching                      | `false`                |
| `--timeout`       | HTTP request timeout in seconds                 | `30`                   |
| `--max-concurrent`| Maximum number of concurrent downloads          | `5`                    |
| `--debug`         | Enable debug logging                            | `false`                |
| `--gofile-enable` | 启用 gofile 下载                                | `true`                 |
| `--gofile-tool`   | gofile-downloader 脚本路径                      | `~/.local/share/south2md/gofile-downloader/gofile-downloader.py` |
| `--gofile-dir`    | gofile 下载目录                                 | `gofile`               |
| `--gofile-token`  | gofile 账号 token                               |                         |
| `--gofile-venv-dir` | gofile 虚拟环境目录                           | `~/.local/share/south2md/py/gofile` |
| `--gofile-skip-existing` | 跳过已存在的 gofile 内容               | `true`                 |

### Gofile Downloader

需要将 `gofile-downloader` repo clone 到 XDG data home（默认 `~/.local/share/south2md`），
并确保 Python 3.10+ 可用。工具会自动创建并管理虚拟环境。

## Configuration

`south2md` uses a layered configuration model:

1. `defaults`
2. `config file` (TOML)
3. `environment variables`
4. `flags / positional args`

Higher layers override lower layers.

Config file discovery order:

1. `--config=/path/to/south2md.toml`
2. `SOUTH2MD_CONFIG=/path/to/south2md.toml`
3. `./south2md.toml`
4. `$XDG_CONFIG_HOME/south2md/config.toml` (or platform user config dir fallback)

Environment variable examples:

- `SOUTH2MD_TID`
- `SOUTH2MD_INPUT`
- `SOUTH2MD_OUTPUT`
- `SOUTH2MD_OFFLINE`
- `SOUTH2MD_COOKIE_FILE`
- `SOUTH2MD_TIMEOUT`
- `SOUTH2MD_MAX_CONCURRENT`
- `SOUTH2MD_DEBUG`
- `SOUTH2MD_GOFILE_ENABLE`

Example:

```sh
SOUTH2MD_TID=2636739 SOUTH2MD_OUTPUT=./exports south2md --debug
```

## Dependencies

`south2md` is built with the help of several open-source libraries:

-   [github.com/spf13/cobra](https://github.com/spf13/cobra) for the command-line interface.
-   [github.com/PuerkitoBio/goquery](https://github.com/PuerkitoBio/goquery) for HTML parsing.
-   [github.com/JohannesKaufmann/html-to-markdown/v2](https://github.com/JohannesKaufmann/html-to-markdown/v2) for Markdown conversion.
-   [github.com/BurntSushi/toml](https://github.com/BurntSushi/toml) for TOML configuration.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
