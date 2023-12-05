# Kurnik.pl chess bot
Bot to play chess on https://kurnik.pl <br>
Requirements: [golang](https://go.dev), [npm](https://nodejs.org) and [uci engine](https://stockfishchess.org/download/).

### How to compile program?
```sh
cd dashboard
npm install # creates node_modules directory
npm run build # creates build directory
cd ..
go build # creates main program
```

### How to configure and run program?
1. Set login and password in `settings.json` file.
2. Set path to UCI engine in `settings.json` (default is /usr/bin/stockfish for stockfish engine).
3. Run program
    ```sh
    ./main  # linux
    .\main  # windows
    ```
4. Process will work in background. 
5. You can see dashboard: http://localhost:8080
