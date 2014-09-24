# NBE-Dot

## 这是干嘛的?

Dot 是 NBE 核心的主节点, Dot 保存着所有应用的信息, 包括版本, 名称, 配置文件地址, 当前系统中的机器节点信息, 以及当前系统的所有容器的信息. 目前是单点状态, 所有的 levi 都会连接同一个 Dot.

## 怎么运行他呢?

Dot 有一个配置文件, `dot.yaml.sample`, 运行的时候只需要重命名为 `dot.yaml`, 然后

    dot -c dot.yaml [-DEBUG]
    
就可以了, `-DEBUG` 指定是否为 debug 模式, debug 模式下会输出一些 debug 信息, 如任务的格式, app.yaml 和 config.yaml 的数据内容等.

一些必要的配置内容是:

* etcd, machines 是一个列表, 里面是所有的 etcd 节点. etcd 在 Dot 中用来保存应用的配置文件, 是必不可少的.
* nginx, 需要指定 Dot 使用的 nginx template 位置, 以及静态文件的源地址和静态文件目标地址. Dot 会用这个 nginx 来处理包含的静态文件. 这里的 `port` 是指每个 host 上的二级 nginx 的端口, 一般我们会默认开放 80, 如果有调整会在这里修改.
* dba, 这里是 DBA 分配数据库和初始化数据库表的接口地址, 如果手动分配, 会有更加tricky的方法, 可以跳过这一步.

## 怎么样让应用跑在上面呢?

### 这是一个应用的 app.yaml 的例子:

    appname: "docker-registry"
    port: 5000
    runtime: "python"
    build: 
        - "pip install -i http://pypi.douban.com/simple/ ./depends/docker-registry-core && pip install -i http://pypi.douban.com/simple/ ../docker-registry && pip install -i http://pypi.douban.com/simple/ mysql-python"
    cmd:
        - "gunicorn -c gunicorn_config.py app:application"
    services:
        - "mysql"
        - "redis"
        
* port: 应用在容器内部的端口, 需要被暴露出来的. 一个应用只暴露一个端口, 如果需要多个, 那么可能你需要考虑一下怎么解耦他们成为多个应用.
* runtime: 运行时环境, 提供 Python, Java 等.
* build: 打包构建镜像的时候需要执行的命令, 可以认为是运行环境初始化的命令, 会做一些依赖安装等操作.
* cmd: 启动容器的命令, 也就是告诉 NBE 用什么样的命令来执行你的容器.
* services: 目前支持 MySQL 和 Redis, 会根据这个来生成不同的 config.yaml.

### 这是一个应用的 config.yaml 的例子:

目前只支持 MySQL 和 Redis 的自动替换配置. 应用需要的任何配置都可以写在这里, 只有 mysql/redis 会被自动替换, 其他配置都是原有的.

    mysql:
        username: "docker-registry"
        password: "3d6#!2ef"
        host: "10.1.201.58"
        port: 3306
        db: docker-registry
    redis:
        host: "10.1.201.5"
        port: 6379
      
* mysql: 本地运行时需要使用的 MySQL 的配置信息
* redis: 本地运行时需要使用的 Redis 的配置信息

给一个 Python 的例子:

    import yaml
    from flask import Flask
    
    app = Flask(__name__)
    env = {}
    with open('config.yaml', 'r') as f:
        env = yaml.load(f)
    mysql_uri = 'mysql://{username}:{password}@{host}:{port}/{db}'.format(**env)
    app.config.update(SQLALCHEMY_DATABASE_URI=mysql_uri)
    
这样应用就绑定上了 MySQL 连接. 不用担心, 上线的时候你读到的 config.yaml 永远是正确的, 只是需要注意路径, 永远是相对代码仓库的第一级.

## 怎么部署 Dot 呢?

目前采用手动部署的方式, 写在 init.d 里. 可以使用 supervisord 也可以使用 nohup 来运行, 只是后者会比较 low 一点, 而前者坑会比较多一些.

## Restful APIs

目前支持的 API 都是对系统进行修改的, 暂时没有权限鉴定.

* Register:

        POST /app/:app/:version appyaml=&configyaml=
        
    appyaml: 必须传
    configyaml: 可以为空
    
* Add Container:

        POST /app/:app/:version/add host=&daemon=
        
    host: 必须传, 说明部署到哪个 host 上
    daemon: 默认为 false, 如果应用以 daemon 模式运行, 那么传 true
    
* Build Image:

        POST /app/:app/:version/build host=&base=&group=
        
    host: 用哪个 host 来运行打包任务
    base: 基于哪个基底镜像
    group: 代码仓库在哪个组下面
    
* Test Image:

        POST /app/:app/:version/test host=
        
    host: 用哪个 host 来运行测试任务
    
* Remove Application:

        ~~DELETE /app/:app/:version host=~~
        POST /app/:app/:version/delete host=
        
    host: 删除这个 host 上的所有对应 app 的容器