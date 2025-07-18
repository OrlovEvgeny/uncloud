# Install CLI

Install the Uncloud command-line utility to manage your machines and deploy apps using `uc` commands. You will run `uc`
locally so choose the appropriate installation method for your operating system.

:::info NOTE

Windows is not natively supported yet, but you can install and run `uc` in a
[WSL](https://learn.microsoft.com/en-us/windows/wsl/) terminal by following the instructions for Linux.
:::

## Homebrew (macOS, Linux)

If you have [Homebrew](https://brew.sh/) package manager installed, this is the recommended installation method on macOS
and Linux:

```shell
brew install psviderski/tap/uncloud
```

To upgrade to the latest version:

```shell
brew upgrade uncloud
```

## Install script (macOS, Linux)

For a quick automated installation, use the install script:

```shell
curl -fsS https://get.uncloud.run/install.sh | sh
```

The script will:

- Detect your operating system and architecture
- Download the appropriate latest binary from [GitHub releases](https://github.com/psviderski/uncloud/releases)
- Install it to `/usr/local/bin/uncloud` using `sudo` (you may need to enter your user password)
- Create a shortcut `uc` in `/usr/local/bin` for convenience

Don't like `curl | sh`? You can download and review the [install script](https://get.uncloud.run/install.sh) first and
then run it:

```shell
curl -fsSO https://get.uncloud.run/install.sh
cat install.sh
sh install.sh
```

## GitHub download (macOS, Linux)

You can manually download and use a pre-built binary from the
[latest release](https://github.com/psviderski/uncloud/releases/latest) on GitHub.

<Tabs>
  <TabItem value="macOS (Apple Silicon)">
    ```shell
    curl -L https://github.com/psviderski/uncloud/releases/latest/download/uncloud_macos_arm64.tar.gz | tar xz
    mv uncloud uc
    ```
  </TabItem>
  <TabItem value="macOS (Intel)">
    ```shell
    curl -L https://github.com/psviderski/uncloud/releases/latest/download/uncloud_macos_amd64.tar.gz | tar xz
    mv uncloud uc
    ```
  </TabItem>
  <TabItem value="Linux (AMD 64-bit)">
    ```shell
    curl -L https://github.com/psviderski/uncloud/releases/latest/download/uncloud_linux_amd64.tar.gz | tar xz
    mv uncloud uc
    ```
  </TabItem>
  <TabItem value="Linux (ARM 64-bit)">
    ```shell
    curl -L https://github.com/psviderski/uncloud/releases/latest/download/uncloud_linux_arm64.tar.gz | tar xz
    mv uncloud uc
    ```
  </TabItem>
</Tabs>

You can use the `./uc` binary directly from the current directory, or move it to a directory in your system's `PATH`
to run it as `uc` from any location.

For example, move it to `/usr/local/bin` which is a common location for user-installed binaries:

```shell
sudo mv ./uc /usr/local/bin
```

## Debian

On a Debian system, you can install Uncloud CLI from an unofficial
[repository](https://debian.griffo.io/) maintained by
[@dariogriffo](https://github.com/dariogriffo):

```shell
curl -sS https://debian.griffo.io/EA0F721D231FDD3A0A17B9AC7808B4DD62C41256.asc | sudo gpg --dearmor --yes -o /etc/apt/trusted.gpg.d/debian.griffo.io.gpg
echo "deb https://debian.griffo.io/apt $(lsb_release -sc 2>/dev/null) main" | sudo tee /etc/apt/sources.list.d/debian.griffo.io.list
apt install -y uncloud
```

Alternatively, you can download `.deb` packages directly from the repository
[releases](https://github.com/dariogriffo/uncloud-debian/releases) page.

## Verify installation

After installation, verify that `uc` command is working:

```shell
uc --version
```

## Next steps

Now that you have `uc` installed, you're ready to:

- [Deploy demo app](./2-deploy-demo-app.md)
