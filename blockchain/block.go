package blockchain

import (
	"bytes"
	"crypto/sha256"

)

type Block struct {
	Hash []byte
	Data []byte
	PrevHash []byte
}


type BlockChain struct {
	Blocks []*Block
}

func DeriveHash(Data []byte, PrevHash []byte)[]byte{
	info := bytes.Join([][]byte{Data, PrevHash}, []byte{})
	hash := sha256.Sum256(info)
	return hash[:]
}

func CreateBlock(data string, prevHash []byte) * Block {
	block := &Block{[]byte{}, []byte(data), prevHash}
	block.Hash = DeriveHash(block.Data, block.PrevHash)
	return block
}

func (chain *BlockChain) AddBlock(data string) {
	prevBlock := chain.Blocks[len(chain.Blocks)-1]
	new := CreateBlock(data, prevBlock.Hash)
	chain.Blocks = append(chain.Blocks, new)
}

func Genesis() *Block{
	return CreateBlock("Genesis", []byte{})
}

func InitBlockChain() *BlockChain{
	return &BlockChain{[]*Block{Genesis()}}
}
