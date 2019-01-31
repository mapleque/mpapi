微信小程序服务端代理
====

项目简介
----

本项目实现了小程序服务端基于微信api的用户登录状态管理以及业务无关的大部分接口逻辑。

同时本项目还实现了一个用户登录认证代理机制，开发者可以通过本项目服务代理微信请求到自己的服务，代理后的请求头中将会带有：

- `User-Id` 用户唯一id
- `User-Openid` 用户在当前小程序下的openid
- `User-Unionid` 用户在当前小程序下的unionid，如果该小程序没有unionid后者未调用过上传敏感信息接口，则该数据为空
- `User-Extra` 用户的额外信息(json)，包括昵称、头像等，如果未调用过上传敏感信息接口，则该数据为空
- `User-Mobile` 用户手机号(json)，如果未调用过上传手机号接口，则该数据为空

以上数据都是通过微信相关api收集所得，免去了开发者接入微信的麻烦，只需要关注自己的产品逻辑实现即可。

> 注意：当用户未登录或者部分信息未通过接口上传过，则对应的数据可能为空值。

使用公开服务
----
- 搜索小程序"微账号"，按提示注册小程序信息。
- 在小程序后台添加
  - request合法域名:`https://mpapi.mapleque.com`
  - uploadFile合法域名:`https://mpapi.mapleque.com`
  - downloadFile合法域名:`https://mpapi.mapleque.com`
- 直接使用`https://mpapi.mapleque.com`下的本项目各接口

私有部署方法
----

- 从Github[下载代码](https://github.com/mapleque/mpapi)
- 创建数据库表`sql/*.sql`
- 编辑配置文件`main/.env.example`为`main/.env`或者设置相应的环境变量
- 编译`go build -o main/service main/main.go`
- 运行`cd main && ./service`
- 根据服务端口，配置相关nginx代理

接口文档
----
- 所有请求必须携带Header`User-App: <appid>`，否则接口会返回错误码。
- 所有Header中带有`User-Token: <token>`的请求都会尝试通过`token`获取用户信息。

### 登陆

> 本接口返回的token可能过期，客户端在使用时需要注意当接口返回token过期错误码时，需要重新请求登陆接口获取新的token。    
> 特别的：当客户端调用`wx.login`接口后务必使用新`code`请求本接口，以刷新用户`sessionKey`，否则后面部分功能将无法正常使用。    

请求路径：`/login`

请求参数：
```
{
  "code": <string>, // 微信登陆接口返回的临时授权code
}
```

返回数据：
```
{
  "status": <int>,
  "data": {
    "token": <string>, // 用户登陆token，后面需要放在Header的Authorization里边请求接口
    "openid": <string>, // 用户的openid，应客户端统计需求添加，可能有未知安全风险
  }
}
```

### 生成二维码

> 需要登陆后请求。    
> 客户端可以通过`wx.downloadFile`接口请求直接获取图片文件。    
> 注意：本接口也可能返回带有错误信息的json，客户端在调试过程中注意查看。    

请求路径: `/wxcode`

请求方法：GET

请求参数：`?scene=&page=&width=&auto_color=&line_color=&is_hyaline=`

请求参数定义和返回数据参考[微信文档](https://developers.weixin.qq.com/miniprogram/dev/api/open-api/qr-code/getWXACodeUnlimit.html)。

### 发送模板通知

> 需要登陆后请求。    
> 当客户端能够确定何时给用户发送何种消息时，可通过本接口注册消息推送信息。    
> 注意各参数必须符合微信的要求，否则发送消息时会失败，本接口并不会验证参数的正确性。    

请求路径：`/notify`

请求参数：
```
{
  "description": <string>, // 当前通知的说明，必填
  "template_id": <string>, // 当前通知模板，必填
  "formid": <string>, // 当前通知所使用的formid，必须是当前用户交互所产生的，必填
  "active_at": <string>, // 当前通知发送时间，格式yyyy-MM-dd HH:mm:ss
  "page": <string>, // 通知点击跳转的页面，参考微信文档
  "data": <string>, // 通知模板填充的数据，参考微信文档
  "emphasis_keyword": <string> // 通知加粗显示的数据，参考微信文档
```

部分请求参数定义参考[微信文档](https://developers.weixin.qq.com/miniprogram/dev/api/open-api/template-message/sendTemplateMessage.html)

返回数据：
```
{
  "status": <int>
}
```

### 上传敏感信息

> 需要登陆后请求。    
> 本接口当前只解密数据并存储，并未验签。    

请求路径: `/credentials`

请求参数：
```
{
  "raw_data": <string>,
  "signature": <string>,
  "encrypted_data": <string>,
  "iv": <string>
}
```

返回数据：
```
{
  "status": <int>
}
```

### 上传手机号

> 需要登陆后请求。    

请求路径: `/mobile`

请求参数：
```
{
  "encrypted_data": <string>,
  "iv": <string>
}
```

返回数据：
```
{
  "status": <int>
}
```

### 接收客服消息
> 将微信小程序的客服消息转接到咨询后台    
> 将本接口链接配置在微信小程序管理后台即可    
> 注意：需要先在后台注册对应的小程序appid和secret

请求路径：`/message_recieve?appid=xxx`

### 回复客服消息
> 用于回复客服消息的接口，通常是在咨询后台配置    
> 注意：该接口需要通过Http Header进行内部认证：`Authorization: <token>`，token请向项目管理员索取。

请求路径：`/message_reply`

请求方法：不限

请求参数：

```
{
  "source": <string>, // 来源用户标识
  "target": <string>, // 目标用户标识
  "type": <string>, // 消息类型，目前支持：text
  "content": <string>, // 消息内容，仅当type为text时生效
}
```

返回数据：
```
{
  "status": <int>,
}
```
