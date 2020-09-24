A simple CLI-script that automatically closes Milestones 
in which all Issues are 100% closed.

It is also possible to send notifications to Rocket.Chat.

Example:

```
go build -o autoclose main.go
./autoclose -gitlaburl http://gitlab.dev -token aaaaBBBBcccc1111 -group 10 
```

Example with Rocket.Chat

```
go build -o autoclose main.go
./autoclose -gitlaburl http://gitlab.dev -token aaaaBBBBcccc1111 -group 10 -rocketurl https://rocket.company.io -user bot -pass botpass -channel "#rocketchannel"
```

Thats all.

PS If you need notify in another messenger, open issue.