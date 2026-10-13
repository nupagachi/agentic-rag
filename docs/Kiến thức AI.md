# Kiến thức AI cho dự án

Oct 9, 2026 · @Nhu Canh

Tài liệu tham khảo các khái niệm RAG và agent dùng trong PRD; sẽ bổ sung dần khi dự án gặp khái niệm mới.

## 1. Chunking

Chunking là cắt tài liệu thành đoạn nhỏ để embed và truy xuất. Đây là bước ảnh hưởng chất lượng RAG nhiều nhất: chunk quá nhỏ thì mất ngữ cảnh, quá lớn thì vector bị “pha loãng” (nhiều chủ đề trong một vector) và tốn token khi đưa vào LLM.

| Chiến lược | Cách làm | Ưu điểm | Nhược điểm | Hợp với |
| --- | --- | --- | --- | --- |
| Cố định theo token (fixed-size) | Cắt mỗi N token (vd 512), chồng lấn M token (vd 64) | Đơn giản, dự đoán được kích thước | Cắt ngang câu, ngang bảng, ngang mục | Văn bản thuần không có cấu trúc |
| Đệ quy theo ký tự tách (recursive) | Thử tách theo đoạn → dòng → câu → từ cho đến khi đủ nhỏ | Giữ câu, đoạn nguyên vẹn hơn | Vẫn không hiểu cấu trúc tài liệu | Fallback mặc định |
| Theo cấu trúc (structure / heading) | Cắt theo heading, mục, điều khoản; mục dài thì cắt tiếp bên trong; gắn đường dẫn heading vào chunk | Chunk trọn ý, trích dẫn đúng mục | Cần parser hiểu bố cục tốt | DOCX, PDF có heading, quy trình, hợp đồng |
| Theo đơn vị tài liệu | Mỗi slide, mỗi nhóm dòng bảng (lặp lại dòng tiêu đề), mỗi trang web | Giữ đơn vị tự nhiên của loại file | Kích thước không đều | PPTX, XLSX, URL |
| Ngữ nghĩa (semantic) | Embed từng câu, cắt khi độ tương đồng giữa 2 câu liên tiếp giảm mạnh | Ranh giới theo ý nghĩa | Tốn chi phí embed thêm, khó kiểm soát | Văn bản dài ít cấu trúc (P2) |
| Parent-child (small-to-big) | Tìm bằng chunk nhỏ (chính xác), đưa cho LLM chunk cha lớn hơn (đủ ngữ cảnh) | Vừa chính xác vừa đủ ngữ cảnh | Lưu 2 lớp, logic phức tạp hơn | Tool `read_document` trong dự án làm việc tương tự |
| Contextual chunking | Dùng LLM viết 1–2 câu tóm tắt ngữ cảnh tài liệu, gắn vào đầu mỗi chunk trước khi embed | Tăng đáng kể độ chính xác truy xuất | Tốn chi phí LLM cho mọi chunk | Cân nhắc ở P1, đo bằng Eval |

**Tham số quan trọng**

- **Kích thước chunk:** thường 256–1.024 token. Câu hỏi tra cứu sự kiện ngắn hợp chunk nhỏ; câu hỏi cần giải thích hợp chunk lớn.
- **Overlap (chồng lấn):** 10–20% kích thước chunk, để ý nằm giữa 2 chunk không bị mất.
- **Metadata gắn kèm:** tên tài liệu, heading, trang, ngôn ngữ — vừa dùng lọc, vừa hiển thị trích dẫn, vừa có thể chèn vào text trước khi embed để vector hiểu ngữ cảnh.
- **Tiếng Việt và tiếng Trung:** đếm token theo tokenizer của model embedding, không đếm theo từ; tiếng Việt và tiếng Trung tốn nhiều token hơn tiếng Anh cho cùng lượng nội dung.

**Dự án chọn:** theo cấu trúc cho DOCX/PDF, theo đơn vị cho PPTX/XLSX/URL, cố định theo token làm fallback; mặc định 512 token, overlap 64. Không có con số đúng cho mọi KB, nên Document Inspector cho xem trước chunk và Eval cho đo kết quả khi đổi cấu hình.

## 2. Embedding, tìm kiếm và rerank

Truy xuất của dự án chạy 3 lớp: tìm bằng ngữ nghĩa (dense), tìm bằng từ khóa (BM25), gộp 2 kết quả (RRF), rồi chấm lại bằng reranker để lấy vài đoạn tốt nhất.

| Khái niệm | Là gì | Vì sao cần |
| --- | --- | --- |
| Embedding (dense vector) | Model biến đoạn văn thành dãy số (vd 1.024 chiều); 2 đoạn cùng ý nghĩa cho 2 vector gần nhau (đo bằng cosine) | Tìm được đoạn cùng ý dù khác từ, kể cả khác ngôn ngữ (hỏi tiếng Việt, tìm ra tiếng Trung) nếu model đa ngôn ngữ |
| BM25 (sparse vector) | Chấm điểm theo từ trùng: từ xuất hiện nhiều trong đoạn (TF) và hiếm trong toàn bộ kho (IDF) thì điểm cao | Bắt tốt mã số, tên riêng, mã sản phẩm, từ viết tắt mà embedding hay bỏ lỡ |
| Tokenizer | Cách cắt văn bản thành “từ” cho BM25 | Tiếng Việt tách theo âm tiết (kèm cặp âm tiết), tiếng Trung không có dấu cách nên tách theo cặp ký tự; phải giống hệt lúc index và lúc hỏi |
| Hybrid search | Chạy cả dense và BM25 rồi gộp | Mỗi loại bù điểm yếu của loại kia; thường tốt hơn dùng một loại |
| RRF (Reciprocal Rank Fusion) | Điểm gộp = tổng 1/(k + thứ hạng) ở mỗi danh sách, thường k = 60 | Gộp theo thứ hạng nên không cần chuẩn hóa 2 thang điểm khác nhau |
| Reranker (cross-encoder) | Model đọc cùng lúc câu hỏi và từng đoạn, cho điểm liên quan | Chính xác hơn embedding nhiều nhưng chậm, nên chỉ chấm lại top 30 rồi giữ top 8 |
| Ngưỡng rerank | Điểm tối thiểu để coi là liên quan | Dưới ngưỡng thì agent nói “không tìm thấy” thay vì bịa |
| top-k | Số đoạn lấy ra ở mỗi bước | k lớn tăng khả năng có đoạn đúng (recall) nhưng tốn token và dễ lẫn đoạn nhiễu |

**Lưu ý khi dùng embedding của provider**

- Vector của model A không so được với model B: đổi model embedding là phải embed lại toàn bộ KB (re-index).
- Một số model phân biệt “embed tài liệu” và “embed câu hỏi” (vd Cohere `input_type`); adapter phải truyền đúng.
- Số chiều lớn hơn thường tốt hơn chút ít nhưng tốn RAM Qdrant; nhiều model cho phép rút gọn số chiều (Matryoshka) với mất mát nhỏ.
- Quantization int8 trong Qdrant giảm RAM khoảng 4 lần, độ chính xác giảm rất ít.
- Chọn model bằng Eval trên chính tài liệu của công ty; bảng xếp hạng công khai (vd MTEB) chỉ để lọc ứng viên.

## 3. Agent

RAG thường tìm một lần rồi trả lời. Agentic RAG để LLM tự quyết định tìm gì, ở đâu, tìm bao nhiêu lần, và khi nào đã đủ để trả lời.

| Khái niệm | Giải thích | Trong dự án |
| --- | --- | --- |
| Tool / function calling | LLM trả về yêu cầu gọi hàm dạng JSON (tên tool + tham số) thay vì văn bản; code của mình chạy hàm rồi gửi kết quả lại | 3 tool: `list_kbs`, `search_kb`, `read_document`; model chat bắt buộc hỗ trợ tool calling |
| ReAct (Reason + Act) | Vòng lặp: suy nghĩ → gọi tool → đọc kết quả → suy nghĩ tiếp → … → trả lời | Agent loop viết bằng Go, hiển thị từng bước cho người dùng (sự kiện `agent.step`) |
| Query rewriting | Viết lại câu hỏi cho rõ, đủ ngữ cảnh (“còn cái kia thì sao?” → câu đầy đủ) | P1, dùng model utility rẻ |
| Query decomposition | Tách câu hỏi phức tạp thành nhiều câu con, tìm từng câu | P1; ví dụ so sánh 2 chính sách |
| Routing | Chọn KB phù hợp dựa trên mô tả KB | Tool `list_kbs` trả mô tả; vì vậy mô tả KB nên viết rõ |
| Giới hạn vòng lặp | Chặn agent lặp vô hạn hoặc tốn token | Tối đa 5 lần gọi tool, 60 giây; hết thì trả lời với những gì đã có |
| Grounding và trích dẫn | Yêu cầu mọi ý phải dựa trên đoạn đã tìm, đánh số \[n\] | Hậu kiểm: \[n\] phải trỏ đoạn đã truy xuất, sai thì bỏ |
| Hallucination (bịa) | LLM tự tin viết điều không có trong tài liệu | Ngưỡng rerank + hướng dẫn từ chối + chỉ số faithfulness trong Eval |
| Memory hội thoại | Đưa lịch sử chat vào prompt để hiểu câu hỏi nối tiếp | Giữ N lượt gần nhất, lượt cũ hơn được tóm tắt để tiết kiệm token |
| Prompt injection | Chỉ dẫn độc nằm trong tài liệu hoặc trang web (“bỏ qua mọi hướng dẫn trước…”) mà LLM đọc phải | Bọc chunk trong thẻ `<document>`, system prompt coi đó là dữ liệu; tool chỉ đọc; quyền KB ép ở code chứ không tin LLM |

**Vì sao quyền không được giao cho LLM:** LLM có thể bị thuyết phục bởi câu hỏi hoặc nội dung tài liệu. Vì vậy tham số `kb_ids` mà LLM đưa ra luôn bị Go giao với danh sách KB người dùng được phép trước khi truy vấn Qdrant.

**Khi nào không cần agent:** câu hỏi đơn giản trong 1 KB thì RAG 1 lượt nhanh và rẻ hơn; dự án giữ chế độ này làm fallback (M5-10) khi model không hỗ trợ tool calling.

## 4. Eval: đo chất lượng RAG

Eval tách lỗi thành 2 tầng: **truy xuất** có lấy đúng đoạn không, và **câu trả lời** có dùng đúng đoạn đó không. Trả lời sai mà truy xuất đúng thì lỗi ở prompt/model; truy xuất sai thì lỗi ở chunking, embedding hoặc rerank.

**Eval dataset** gồm: câu hỏi, đáp án chuẩn, danh sách chunk/tài liệu chứa câu trả lời. Nên có 100–300 câu mỗi KB, trộn 4 loại: tra cứu sự kiện, cần ghép nhiều đoạn, hỏi xuyên ngôn ngữ, và câu **không có** trong tài liệu (để đo việc từ chối). Nguồn: chuyên gia viết tay, LLM sinh từ chunk rồi người duyệt, câu bị dislike trong chat.

### 4.1 Chỉ số truy xuất (tính bằng code, không cần LLM)

| Chỉ số | Đo gì | Cách tính |
| --- | --- | --- |
| Hit@k | Có ít nhất 1 đoạn đúng trong top k không | % câu hỏi có ít nhất 1 chunk đúng trong k kết quả đầu |
| Recall@k | Lấy được bao nhiêu phần các đoạn cần thiết | Số chunk đúng trong top k ÷ tổng số chunk đúng, trung bình các câu |
| Precision@k | Bao nhiêu phần kết quả là đúng | Số chunk đúng trong top k ÷ k |
| MRR | Đoạn đúng đầu tiên nằm cao hay thấp | Trung bình của 1 / vị trí chunk đúng đầu tiên (vị trí 1 → 1; vị trí 3 → 0,33) |
| nDCG@k | Thứ tự xếp hạng tốt đến đâu, phạt đoạn đúng nằm thấp | DCG = tổng (độ liên quan ÷ log₂(vị trí + 1)), chia cho DCG của thứ tự lý tưởng |

Ví dụ: 200 câu, 172 câu có chunk đúng trong top 5 → Hit@5 = 86%. So sánh trước và sau rerank cho biết reranker có đáng tiền không.

### 4.2 Chỉ số câu trả lời (dùng LLM làm giám khảo)

| Chỉ số | Đo gì | Cách tính |
| --- | --- | --- |
| Faithfulness (trung thành) | Câu trả lời có bịa ngoài ngữ cảnh không | LLM tách câu trả lời thành các ý, kiểm từng ý có được các chunk hỗ trợ không; điểm = số ý được hỗ trợ ÷ tổng số ý |
| Answer relevance | Có trả lời đúng câu được hỏi không | LLM chấm 1–5 hoặc sinh lại câu hỏi từ câu trả lời và so với câu gốc |
| Answer correctness | Đúng so với đáp án chuẩn không | LLM so câu trả lời với đáp án chuẩn: đúng / đúng một phần / sai |
| Citation accuracy | Trích dẫn \[n\] có thật sự chứa ý được nêu không | Với mỗi cặp (ý, \[n\]), LLM kiểm chunk n có hỗ trợ ý đó không |
| Refusal accuracy | Với câu không có trong tài liệu, agent có nói “không tìm thấy” không | % câu “không có đáp án” mà agent từ chối đúng (và % câu có đáp án mà agent từ chối nhầm) |

**LLM-as-judge cần lưu ý:** dùng model giám khảo mạnh và cố định giữa các lần chạy; prompt chấm có tiêu chí rõ và ví dụ; đặt temperature = 0; định kỳ lấy 30–50 câu cho người chấm để kiểm giám khảo có đồng ý với người không. Giám khảo có xu hướng thiên vị câu trả lời dài và câu trả lời do chính nó sinh ra.

### 4.3 Quy trình dùng Eval

1. Chạy baseline với cấu hình hiện tại, lưu snapshot cấu hình.
2. Mỗi lần chỉ đổi **một** thứ (chunk size, model embedding, top-k, prompt…) rồi chạy lại.
3. So sánh 2 lần chạy: chỉ số tăng/giảm và danh sách câu đổi kết quả; đọc trực tiếp các câu tệ đi.
4. Chỉ áp dụng khi cải thiện rõ và chi phí chấp nhận được; chỉ số tụt quá ngưỡng (mặc định 5 điểm %) thì cảnh báo.

Mục tiêu của dự án (PRD 9.1): Hit@5 ≥ 85%, faithfulness ≥ 90%, citation accuracy ≥ 95%, refusal accuracy ≥ 90%. Bộ mẫu có sẵn: RAGAS (Python) định nghĩa các chỉ số tương tự, có thể tham khảo prompt chấm khi viết worker-eval.

## 5. Token, chi phí và thuật ngữ khác

| Thuật ngữ | Giải thích | Ảnh hưởng tới dự án |
| --- | --- | --- |
| Token | Đơn vị văn bản model xử lý (một phần của từ). Tiếng Anh khoảng 0,75 từ/token; tiếng Việt và tiếng Trung tốn nhiều token hơn | Provider tính tiền theo token vào và token ra; gateway đếm và ghi `model_usage` |
| Context window | Số token tối đa model đọc được trong 1 lần gọi (prompt + lịch sử + chunk + câu trả lời) | Giới hạn số chunk và độ dài lịch sử đưa vào; nhiều ngữ cảnh hơn không luôn tốt hơn (model dễ bỏ sót thông tin nằm giữa) |
| TTFT (time to first token) | Thời gian từ lúc gửi tới lúc nhận token đầu tiên | Chỉ số cảm nhận tốc độ chính của chat; mục tiêu dưới 3 giây |
| Streaming | Nhận câu trả lời từng đoạn thay vì chờ xong | Dùng SSE từ provider → Go → trình duyệt |
| Temperature | Mức ngẫu nhiên khi sinh văn bản | RAG và giám khảo Eval dùng thấp (0–0,3) để ổn định, bám tài liệu |
| Rate limit (RPM, TPM) | Số request và số token mỗi phút provider cho phép | Nút thắt cổ chai khi embed hàng loạt; gateway chia hạn mức, ưu tiên chat |
| Prompt caching | Provider giảm giá / tăng tốc khi phần đầu prompt lặp lại | Đặt system prompt và hướng dẫn cố định ở đầu prompt để tận dụng |
| Semantic cache | Lưu câu trả lời cho câu hỏi gần giống đã gặp | Giảm chi phí (P1); phải xóa khi KB thay đổi và tôn trọng quyền |
| Zero data retention | Cam kết của provider không lưu dữ liệu request | Yếu tố chọn provider vì tài liệu nội bộ được gửi đi |

**Ước chi phí nhanh:** chi phí embed ban đầu ≈ tổng token của mọi chunk × giá embed; chi phí mỗi câu hỏi ≈ (system prompt + lịch sử + top-n chunk) × giá token vào × số vòng agent + độ dài câu trả lời × giá token ra + chi phí rerank. Vì agent có thể gọi model nhiều vòng, chi phí mỗi câu hỏi có thể gấp 2–4 lần RAG 1 lượt; Dashboard chi phí giúp theo dõi điều này.
