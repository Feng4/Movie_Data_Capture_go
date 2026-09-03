# 分片文件处理

同一部影片拆成多个文件时（CD1/CD2、Part1/Part2 等），MDC-Go 自动分组：**只刮削一次、归档到同一目录**。

## 识别的格式

| 格式 | 示例 |
|------|------|
| CD | `SSIS-001-cd1.mp4`, `SSIS-001-cd2.mp4` |
| Part | `SSIS-001_part_1.mp4`, `SSIS-001_part_2.mp4` |
| 数字下划线 | `SSIS-001_1.mp4`, `SSIS-001_2.mp4` |
| 字母 | `SSIS-001-A.mp4`, `SSIS-001-B.mp4` |
| Disc | `SSIS-001-disc1.mkv`, `SSIS-001-disc2.mkv` |

## 处理流程

1. **分组**: 扫描时按基础名（番号）归类，识别分片序号
2. **去重**: 以组为单位进入刮削队列，整组只请求一次网络数据
3. **归档**: 组内所有文件移动/链接到同一输出目录
4. **改名**: 统一为 `番号-1`、`番号-2`…（保留属性后缀，如 `SSIS-001-C-1.mp4`）
5. **NFO**: 单个 NFO 记录总分片数、各分片文件名与总体积

```
输入:
/movies/SSIS-001-cd1.mp4
/movies/SSIS-001-cd2.mp4

输出:
JAV_output/演员/SSIS-001/
├── SSIS-001-1.mp4
├── SSIS-001-2.mp4
├── SSIS-001.nfo        # 记录 2 分片
├── poster.jpg
└── fanart.jpg
```

## 注意事项

- **命名一致性**: 同组文件的番号部分必须一致，分片标志要规范（`-cd1` 优于 `第一集`）
- **缺片容错**: 组内缺某个分片时仍会处理，日志会有警告（`has missing parts, processing anyway`）
- **图片只下一次**: 海报、剧照等只按组下载一份
- **扩展名可混用**: 同组的 `.mp4` 与 `.mkv` 可共存，各自保留原扩展名
- **分片跳过失败列表**: 分析/链接模式下分片组同样受失败列表控制

## 识别失败的排查

```bash
# 打开调试查看分组日志
./mdc -debug -path "/movies" -logdir "./logs"
# 关注 "Found N fragment groups" 与 "Fragment group 'xxx'" 行
```

分组不符预期时，按上面的标准格式重命名文件即可。

## 相关

- [命名与输出](Naming-and-Output.md)
- [命令行参数](CLI-Reference.md)
