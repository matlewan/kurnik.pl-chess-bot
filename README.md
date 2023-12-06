# Kurnik.pl chess bot
Bot to play chess on https://kurnik.pl <br>

### How to configure and run program in linux?
1. Download [release](https://github.com/matlewan/kurnik.pl-chess-bot/releases/download/1.0/kurnik.pl-chess-bot-linux-x64.tar.gz).
2. Unpack archive.
3. Set login and password in `settings.json` file.
4. Run program
    ```sh
    ./main  # linux
    ```
5. Process will work in background. 
6. You can see dashboard: http://localhost:8080

### Development
Requirements: [golang](https://go.dev), [npm](https://nodejs.org) and [uci engine](https://stockfishchess.org/download/).

How to compile program?
```sh
go build
go run main
cd dashboard
npm ci # creates node_modules directory
npm start # creates build directory
```
