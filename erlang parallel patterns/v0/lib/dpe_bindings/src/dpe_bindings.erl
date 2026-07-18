-module(dpe_bindings).
-export([submit_spark_job/3, submit_flink_job/3]).

%% Function to submit a job to Spark
submit_spark_job(SparkUrl, JobFun, Args) ->
    SerializedJob = serialize_job(JobFun, Args),
    RequestBody = create_request_body(SerializedJob),
    case httpc:request(post, {SparkUrl, [], "application/json", RequestBody}, [], []) of
        {ok, {_, 200, ResponseBody}} ->
            {ok, deserialize_response(ResponseBody)};
        {ok, {_, StatusCode, _}} ->
            {error, {spark_job_failed, StatusCode}};
        {error, Reason} ->
            {error, {spark_job_failed, Reason}}
    end.

%% Function to submit a job to Flink
submit_flink_job(FlinkUrl, JobFun, Args) ->
    SerializedJob = serialize_job(JobFun, Args),
    RequestBody = create_request_body(SerializedJob),
    case httpc:request(post, {FlinkUrl, [], "application/json", RequestBody}, [], []) of
        {ok, {_, 200, ResponseBody}} ->
            {ok, deserialize_response(ResponseBody)};
        {ok, {_, StatusCode, _}} ->
            {error, {flink_job_failed, StatusCode}};
        {error, Reason} ->
            {error, {flink_job_failed, Reason}}
    end.

%% Helper function to serialize the Erlang job
serialize_job(JobFun, Args) ->
    term_to_binary({JobFun, Args}).

%% Helper function to create a request body
create_request_body(SerializedJob) ->
    %% Converts the serialized job to a JSON object or suitable format
    %% for the Python worker to process.
    %% base64 encoding to safely transmit the binary data.
    Base64Job = base64:encode(SerializedJob),
    Json = io_lib:format("{\"job\":\"~s\"}", [Base64Job]),
    lists:flatten(Json).

%% Helper function to deserialize the response
deserialize_response(ResponseBody) ->
    %% response is base64 encoded and needs to be decoded and deserialized.
    {ok, DecodedResponse} = base64:decode(ResponseBody),
    binary_to_term(DecodedResponse).