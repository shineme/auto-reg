> For the complete documentation index, see [llms.txt](https://help.cliproxy.com/llms.txt). Markdown versions of documentation pages are available by appending `.md` to page URLs; this page is available as [Markdown](https://help.cliproxy.com/zh/faq/traffic-api.md).

# Cliproxy流量套餐如何快速集成使用

在Cliproxy中，代理的连接参数（主机、端口、账号、密码）具有特定的逻辑，以实现通过中间服务器动态选择出口 IP。

#### 主机与端口

**主机（Host）：** 连接的 Socks5 中间服务器 IP 或域名。

**端口（Port）：** 中间服务器监听的 Socks5 端口。

**作用：**

* 所有客户端均连接到相同的中间服务器。
* 该服务器充当代理请求的转发中心。

**特别说明**：目前仅提供**us.cliproxy.io**(美国)和**sg.cliproxy.io**(新加坡)作为中间服务器，请选择距离出口IP所在地区较近的中间服务器

<figure><img src="https://1475040396-files.gitbook.io/~/files/v0/b/gitbook-x-prod.appspot.com/o/spaces%2FDmmzUJyPG4sQRjCt7KZ7%2Fuploads%2FFNP1UahuOcjYrQYKrsIv%2Fimage.png?alt=media&amp;token=0cec7fda-28c9-4b60-9494-65eb3287deda" alt=""><figcaption></figcaption></figure>

#### 账号

**格式1(Sticky IP):**`username-region-地区缩写-st-州省名-sid-当前会话唯一标识-t-IP持续时间(`例如：test123456-region-US-st-Louisiana-sid-e6CXdJDm-t-15)

<figure><img src="https://1475040396-files.gitbook.io/~/files/v0/b/gitbook-x-prod.appspot.com/o/spaces%2FDmmzUJyPG4sQRjCt7KZ7%2Fuploads%2FoyndxdMOFWaWj6xQRZ6Q%2Fimage.png?alt=media&amp;token=4b5fa3a9-f67d-41b1-b7ba-961f4b43a2f9" alt=""><figcaption></figcaption></figure>

**格式2(Rotating IP)**:`username-region-地区缩写-st-州省名(`例如：test123456-region-US-st-Louisiana)

<figure><img src="https://1475040396-files.gitbook.io/~/files/v0/b/gitbook-x-prod.appspot.com/o/spaces%2FDmmzUJyPG4sQRjCt7KZ7%2Fuploads%2F9pQlpC8Xl5zC14gYjPTA%2Fimage.png?alt=media&amp;token=bc2debd0-bfdd-4fc9-8e89-59070690d666" alt=""><figcaption></figcaption></figure>

**作用：**

* 账号部分 (`username`) 用于识别用户身份。
* 服务器解析 `username` 后`region` `st`的参数，并根据配置选择合适的出口。

**特别说明**：

* 所有参数以“-”分割，其中**username**以及**region**为**必填**项，其他参数可为空。例如格式1可以为**test123456-region-US-t-15** 格式2可以为**test123456-region-US**
* **当前会话标识(sid)**&#x66F4;改则会判定为新请求，中间服务器会自动分配一条**新IP**
* **国家代码参考**[ISO 3166-1](https://cliproxy.com/zh/iso-code/)国际标准

#### 密码

**作用：**&#x7528;于用户身份认证。不影响出口 IP 选择，仅用于安全校验。

#### 逻辑流程

**客户端连接 Socks5 服务器**

* 使用指定的主机:端口进行连接。

**服务器解析账号信息**

* 提取 `username` `地区参数` `会话类型`信息
* 验证 `username` 是否有效。
* 根据`地区参数`查找 `出口IP` 是否可用（若未指定，则使用默认策略分配）。

**密码认证**

* 服务器校验 `password` 是否正确。
* 若认证失败，则拒绝连接。

**出口 IP 选择**

* 若账号中明确指定了 `出口IP地理位置`，则使用该 地理位置IP 作为出口。
* 若未指定 `地理位置`，则根据预定义策略（如轮询、负载均衡等）分配出口 IP。

**连接目标服务器**

* 服务器根据解析的出口 IP，向目标地址发起连接，并转发流量。

#### 典型示例

| Cliproxy账号                                          | 连接出口IP所在地    | 备注                                      |
| --------------------------------------------------- | ------------ | --------------------------------------- |
| test123456-region-US-st-Louisiana-sid-e6CXdJDm-t-15 | 美国Louisiana州 | 当sid切换，出口IP也会切换。当sid不变，则出口IP持续15分钟后自动切换 |
| test123456-region-SG-t-15                           | 新加坡          | 空                                       |
| test123456-region-HK                                | 香港           | 每次请求都会以一个新的出口IP连接目标网站                   |
