** Uploading file (upto 50mb zip file supported)

ex. upload a zip file and extract it to `/var/www/html/d2i`

`curl -H "X-TOKEN: <token>" -X POST -F 'file=@<path_to_dist>.zip' -F 'args=/var/www/html/d2i' http://localhost:21000/csync`

** Unzipping uploaded file

ex. unzipping uploaded dist.zip to /var/www/html/d2i directory

`curl -H "X-TOKEN: <token>" -X POST -F 'args=dist.zip|-d|/var/www/html/d2i' http://localhost:21000/unzip`

** Starting/Stopping/Re-Starting service

`curl -H "X-TOKEN: <token>" -X POST -F 'args=start|node.service' http://localhost:21000/systemctl`
`curl -H "X-TOKEN: <token>" -X POST -F 'args=stop|node.service' http://localhost:21000/systemctl`
`curl -H "X-TOKEN: <token>" -X POST -F 'args=restart|node.service' http://localhost:21000/systemctl`

** Removing a directory

ex. removing a directory `/var/www/html/d2i` (deletion allowed only in `/var/www/html` folder due to security reasons)

`curl -H "X-TOKEN: <token>" -X POST -F 'args=/var/www/html/d2i' http://localhost:21000/rmdir`


** Generic command execution (only few shell commands are supported for security reasons)

ex. Executing shell command `ls`

`curl -H "X-TOKEN: <token>" -X POST http://localhost:21000/ls`

ex. Executing shell command `rm` and delete some file `/var/www/html/d2i/sample.txt`

`curl -H "X-TOKEN: <token>" -F 'args=/var/www/html/d2i/sample.txt' -X POST http://localhost:21000/rm`


