# north2md

`north2md` is a command-line tool written in Go to scrape posts from the South Plus forum and convert them into clean, readable Markdown files. It can fetch posts directly by their thread ID, parse local HTML files, and download all associated images and attachments.

## Features

- **Fetch Online Posts**: Scrape forum posts directly by providing a thread ID (TID).
- **Parse Local Files**: Convert locally saved HTML files into Markdown.
- **Attachment Downloading**: Automatically download and cache images and other attachments from the post.
- **Cookie-Based Authentication**: Use your browser's cookies to access restricted or members-only content.
- **Markdown Formatting**: Generate well-formatted Markdown with options to include author information, table of contents, and more.
- **Configurable**: Customize the tool's behavior through command-line flags or a TOML configuration file.

## Installation

To install `north2md`, you need to have Go installed on your system. You can then build and install the tool using the following command:

```sh
go install github.com/fdkevin0/north2md@latest
```

## Usage

### Fetching an Online Post

To fetch a post, simply provide the thread ID (TID) as an argument:

```sh
north2md <TID>
```

For example:

```sh
north2md 2636739 --output=post.md
```

### Parsing a Local HTML File

If you have a post saved as an HTML file, you can parse it using the `--input` flag:

```sh
north2md --input=post.html --output=post.md
```

### Using Cookies for Authentication

To access restricted content, you can use a cookie file. The tool supports importing cookies from a `curl` command.

1.  **Import Cookies**:
    First, import your cookies from a `curl` command. You can get this command from your browser's developer tools (Network tab -> right-click on a request -> Copy as cURL).

    ```sh
    north2md cookie import --curl="<your-curl-command>"
    ```

    This will create a `cookies.toml` file in the current directory.

2.  **Fetch with Cookies**:
    Now, you can use the `--cookie-file` flag to fetch the post:

    ```sh
    north2md 2636739 --cookie-file=./cookies.toml --output=post.md
    ```

### Command-Line Flags

Here are all the available command-line flags:

| Flag              | Description                                     | Default                |
| ----------------- | ----------------------------------------------- | ---------------------- |
| `--tid`           | Thread ID (for online fetching)                 |                        |
| `--input`         | Input HTML file path                            |                        |
| `--output`        | Output Markdown file path                       | `post.md`              |
| `--cache-dir`     | Directory for caching attachments               | `./cache`              |
| `--base-url`      | Base URL of the forum                           | `https://north-plus.net/` |
| `--cookie-file`   | Path to the cookie file                         | `./cookies.toml`       |
| `--no-cache`      | Disable attachment caching                      | `false`                |
| `--timeout`       | HTTP request timeout in seconds                 | `30`                   |
| `--max-concurrent`| Maximum number of concurrent downloads          | `5`                    |
| `--debug`         | Enable debug logging                            | `false`                |

## Configuration

In addition to command-line flags, `north2md` can be configured via a TOML file. The configuration options correspond to the command-line flags and allow for more advanced customization of selectors and formatting.

## Dependencies

`north2md` is built with the help of several open-source libraries:

-   [github.com/spf13/cobra](https://github.com/spf13/cobra) for the command-line interface.
-   [github.com/PuerkitoBio/goquery](https://github.com/PuerkitoBio/goquery) for HTML parsing.
-   [github.com/JohannesKaufmann/html-to-markdown/v2](https://github.com/JohannesKaufmann/html-to-markdown/v2) for Markdown conversion.
-   [github.com/BurntSushi/toml](https://github.com/BurntSushi/toml) for TOML configuration.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
