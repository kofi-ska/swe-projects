import os
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from langchain.llms import OpenAI
from langchain.embeddings import HuggingFaceEmbeddings
from langchain.vectorstores import FAISS
from langchain.chains import RetrievalQA
from langchain.document_loaders import TextLoader
from langchain.text_splitter import CharacterTextSplitter
from transformers import AutoTokenizer
import numpy as np
import requests

app = FastAPI()

# Initialize LLM
llm = OpenAI(temperature=0.7)

# Initialize embeddings
embeddings = HuggingFaceEmbeddings()

# Initialize tokenizer
tokenizer = AutoTokenizer.from_pretrained("bert-base-uncased")

# Vector DB setup
VECTOR_DB_PATH = "./vector_db"

def initialize_vector_db():
    if os.path.exists(VECTOR_DB_PATH):
        return FAISS.load_local(VECTOR_DB_PATH, embeddings)
    else:
        # Load and process documents
        loader = TextLoader("./data/erlang_docs.txt")
        documents = loader.load()
        text_splitter = CharacterTextSplitter(chunk_size=1000, chunk_overlap=0)
        texts = text_splitter.split_documents(documents)

        # Create and save the vector store
        vector_store = FAISS.from_documents(texts, embeddings)
        vector_store.save_local(VECTOR_DB_PATH)
        return vector_store

vector_db = initialize_vector_db()

def update_vector_db(new_document_path):
    loader = TextLoader(new_document_path)
    documents = loader.load()
    text_splitter = CharacterTextSplitter(chunk_size=1000, chunk_overlap=0)
    texts = text_splitter.split_documents(documents)
    
    global vector_db
    vector_db.add_documents(texts)
    vector_db.save_local(VECTOR_DB_PATH)

class Query(BaseModel):
    text: str

@app.post("/process_query")
async def process_query(query: Query):
    try:
        # Tokenize the query
        tokens = tokenizer.encode(query.text, return_tensors="pt")
        
        # Retrieve relevant information from vector database
        retriever = vector_db.as_retriever()
        qa_chain = RetrievalQA.from_chain_type(llm=llm, chain_type="stuff", retriever=retriever)
        result = qa_chain({"query": query.text})
        
        # Generate farm pattern code
        farm_pattern = generate_farm_pattern(result['result'], query.text)
        
        # Generate sequence of operations
        sequence = generate_sequence(result['result'])
        
        return {
            "farm_pattern": farm_pattern,
            "sequence": sequence
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

def generate_farm_pattern(retrieved_info, user_query):
    prompt = f"Generate Erlang farm pattern code for: {user_query}\nBased on: {retrieved_info}"
    response = llm(prompt)
    return response

def generate_sequence(retrieved_info):
    prompt = f"Generate a sequence of operations based on: {retrieved_info}"
    response = llm(prompt)
    return response

@app.post("/update_db")
async def update_db(file_path: str):
    try:
        update_vector_db(file_path)
        return {"message": "Vector database updated successfully"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
