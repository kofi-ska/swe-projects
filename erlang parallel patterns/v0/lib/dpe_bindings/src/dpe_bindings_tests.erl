-module(dpe_bindings_tests).
-include_lib("eunit/include/eunit.hrl").

%% Mock functions
%% Mock function for simulating an HTTP request response
mock_http_request(post, {_, _, _, _}, _, _) ->
    {ok, {{"HTTP/1.1", 200, "OK"}, [], base64:encode(term_to_binary({ok, result}))}}.

%% Mock function for simulating an HTTP request error
mock_http_request_error(post, {_, _, _, _}, _, _) ->
    {error, reason}.

%% Test cases
%% Test case for submitting a job to Spark
submit_spark_job_test() ->
    % Set up mock for httpc:request/4
    meck:new(httpc),
    meck:expect(httpc, request, fun mock_http_request/4),
    % Call the submit_spark_job function
    Result = dpe_bindings:submit_spark_job("http://spark-master:8080", fun() -> ok end, []),
    % Assert the result is as expected
    ?assertEqual({ok, {ok, result}}, Result),
    % Clean up mock
    meck:unload(httpc).

%% Test case for submitting a job to Flink
submit_flink_job_test() ->
    % Set up mock for httpc:request/4
    meck:new(httpc),
    meck:expect(httpc, request, fun mock_http_request/4),
    % Call the submit_flink_job function
    Result = dpe_bindings:submit_flink_job("http://flink-jobmanager:8081", fun() -> ok end, []),
    % Assert the result is as expected
    ?assertEqual({ok, {ok, result}}, Result),
    % Clean up mock
    meck:unload(httpc).

%% Test case for handling errors when submitting a job to Spark
submit_spark_job_error_test() ->
    % Set up mock for httpc:request/4 to return an error
    meck:new(httpc),
    meck:expect(httpc, request, fun mock_http_request_error/4),
    % Call the submit_spark_job function
    Result = dpe_bindings:submit_spark_job("http://spark-master:8080", fun() -> ok end, []),
    % Assert that an error is returned as expected
    ?assertMatch({error, {spark_job_failed, reason}}, Result),
    % Clean up mock
    meck:unload(httpc).

%% Test case for handling errors when submitting a job to Flink
submit_flink_job_error_test() ->
    % Set up mock for httpc:request/4 to return an error
    meck:new(httpc),
    meck:expect(httpc, request, fun mock_http_request_error/4),
    % Call the submit_flink_job function
    Result = dpe_bindings:submit_flink_job("http://flink-jobmanager:8081", fun() -> ok end, []),
    % Assert that an error is returned as expected
    ?assertMatch({error, {flink_job_failed, reason}}, Result),
    % Clean up mock
    meck:unload(httpc).

%% Test case for serializing a job
serialize_job_test() ->
    Job = fun() -> ok end,
    Args = [1, 2, 3],
    % Call serialize_job function
    Serialized = dpe_bindings:serialize_job(Job, Args),
    % Assert that the deserialized binary matches the original job and arguments
    ?assertEqual({Job, Args}, binary_to_term(Serialized)).

%% Test case for creating a request body
create_request_body_test() ->
    SerializedJob = term_to_binary({fun() -> ok end, [1, 2, 3]}),
    % Call create_request_body function
    RequestBody = dpe_bindings:create_request_body(SerializedJob),
    % Assert that the request body contains the expected JSON format
    ?assertMatch(<<"{\"job\":\"", _/binary>>, list_to_binary(RequestBody)).

%% Test case for deserializing a response
deserialize_response_test() ->
    Response = base64:encode(term_to_binary({ok, result})),
    % Call deserialize_response function
    Deserialized = dpe_bindings:deserialize_response(Response),
    % Assert that the deserialized response matches the original result
    ?assertEqual({ok, result}, Deserialized).
